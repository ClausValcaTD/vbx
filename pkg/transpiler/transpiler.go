package transpiler

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

type DataType string

const (
	TypeInt     DataType = "long long"
	TypeDouble  DataType = "double"
	TypeString  DataType = "const char*"
	TypeUnknown DataType = "unknown"
)

// Simple expression AST node & parser to evaluate types and transpile C expressions.
type ExprNode interface {
	ExprType(env map[string]DataType) (DataType, error)
	ToC(env map[string]DataType) (string, error)
}

type NumberNode struct {
	Value   string
	IsFloat bool
}

func (n *NumberNode) ExprType(env map[string]DataType) (DataType, error) {
	if n.IsFloat {
		return TypeDouble, nil
	}
	return TypeInt, nil
}

func (n *NumberNode) ToC(env map[string]DataType) (string, error) {
	if !n.IsFloat {
		return n.Value + "LL", nil
	}
	return n.Value, nil
}

type StringNode struct {
	Value string
}

func (s *StringNode) ExprType(env map[string]DataType) (DataType, error) {
	return TypeString, nil
}

func (s *StringNode) ToC(env map[string]DataType) (string, error) {
	return `"` + s.Value + `"`, nil
}

type VarNode struct {
	Name string
}

func (v *VarNode) ExprType(env map[string]DataType) (DataType, error) {
	t, ok := env[v.Name]
	if !ok {
		return TypeUnknown, fmt.Errorf("undefined variable: %s", v.Name)
	}
	return t, nil
}

func (v *VarNode) ToC(env map[string]DataType) (string, error) {
	if _, ok := env[v.Name]; !ok {
		return "", fmt.Errorf("undefined variable: %s", v.Name)
	}
	return v.Name, nil
}

type BinaryNode struct {
	Left  ExprNode
	Op    string
	Right ExprNode
}

func isComparisonOp(op string) bool {
	switch op {
	case "==", "=", "!=", "<>", "<", "<=", ">", ">=":
		return true
	default:
		return false
	}
}

func (b *BinaryNode) ExprType(env map[string]DataType) (DataType, error) {
	lt, err := b.Left.ExprType(env)
	if err != nil {
		return TypeUnknown, err
	}
	rt, err := b.Right.ExprType(env)
	if err != nil {
		return TypeUnknown, err
	}

	if isComparisonOp(b.Op) {
		if lt == TypeString && rt == TypeString {
			return TypeInt, nil
		}
		if (lt == TypeInt || lt == TypeDouble) && (rt == TypeInt || rt == TypeDouble) {
			return TypeInt, nil
		}
		return TypeUnknown, fmt.Errorf("incompatible types for comparison %s: %s and %s", b.Op, lt, rt)
	}

	if b.Op == "%" {
		if lt == TypeInt && rt == TypeInt {
			return TypeInt, nil
		}
		return TypeUnknown, fmt.Errorf("incompatible types for modulo operator %%: %s and %s", lt, rt)
	}

	if b.Op == "+" {
		if lt == TypeString || rt == TypeString {
			return TypeString, nil
		}
	}

	if lt == TypeDouble || rt == TypeDouble {
		return TypeDouble, nil
	}
	if lt == TypeInt && rt == TypeInt {
		return TypeInt, nil
	}
	return TypeUnknown, fmt.Errorf("incompatible types for operator %s: %s and %s", b.Op, lt, rt)
}

func (b *BinaryNode) ToC(env map[string]DataType) (string, error) {
	lt, err := b.Left.ExprType(env)
	if err != nil {
		return "", err
	}
	rt, err := b.Right.ExprType(env)
	if err != nil {
		return "", err
	}

	if isComparisonOp(b.Op) {
		if lt == TypeString || rt == TypeString {
			leftC, err := formatStringArg(b.Left, env)
			if err != nil {
				return "", err
			}
			rightC, err := formatStringArg(b.Right, env)
			if err != nil {
				return "", err
			}
			cOp := b.Op
			switch b.Op {
			case "=":
				cOp = "=="
			case "<>":
				cOp = "!="
			}
			return fmt.Sprintf("(strcmp(%s, %s) %s 0)", leftC, rightC, cOp), nil
		}

		leftC, err := b.Left.ToC(env)
		if err != nil {
			return "", err
		}
		rightC, err := b.Right.ToC(env)
		if err != nil {
			return "", err
		}
		cOp := b.Op
		switch b.Op {
		case "=":
			cOp = "=="
		case "<>":
			cOp = "!="
		}
		return fmt.Sprintf("(%s %s %s)", leftC, cOp, rightC), nil
	}

	targetType, err := b.ExprType(env)
	if err != nil {
		return "", err
	}

	if targetType == TypeString {
		// String concatenation using snprintf helper or standard concat helper
		leftC, err := formatStringArg(b.Left, env)
		if err != nil {
			return "", err
		}
		rightC, err := formatStringArg(b.Right, env)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("vbx_concat(%s, %s)", leftC, rightC), nil
	}

	leftC, err := b.Left.ToC(env)
	if err != nil {
		return "", err
	}
	rightC, err := b.Right.ToC(env)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("(%s %s %s)", leftC, b.Op, rightC), nil
}

func formatStringArg(node ExprNode, env map[string]DataType) (string, error) {
	t, err := node.ExprType(env)
	if err != nil {
		return "", err
	}
	cCode, err := node.ToC(env)
	if err != nil {
		return "", err
	}

	switch t {
	case TypeString:
		return cCode, nil
	case TypeInt:
		return fmt.Sprintf("vbx_int_to_str(%s)", cCode), nil
	case TypeDouble:
		return fmt.Sprintf("vbx_double_to_str(%s)", cCode), nil
	default:
		return "", fmt.Errorf("unsupported type for string conversion: %s", t)
	}
}

// Tokenizer & Recursive Descent Parser for Expressions
type TokenType int

const (
	TokNumber TokenType = iota
	TokString
	TokIdent
	TokOp
	TokLParen
	TokRParen
	TokEOF
)

type Token struct {
	Type TokenType
	Val  string
}

func tokenizeExpr(input string) ([]Token, error) {
	var tokens []Token
	i := 0
	n := len(input)

	for i < n {
		ch := input[i]
		if ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n' {
			i++
			continue
		}

		if ch == '"' {
			i++
			start := i
			for i < n && input[i] != '"' {
				if input[i] == '\\' && i+1 < n {
					i++
				}
				i++
			}
			if i >= n {
				return nil, fmt.Errorf("unterminated string literal")
			}
			tokens = append(tokens, Token{Type: TokString, Val: input[start:i]})
			i++ // skip ending double quote
			continue
		}

		if (ch >= '0' && ch <= '9') || ch == '.' {
			start := i
			hasDot := false
			for i < n && ((input[i] >= '0' && input[i] <= '9') || input[i] == '.') {
				if input[i] == '.' {
					if hasDot {
						break
					}
					hasDot = true
				}
				i++
			}
			tokens = append(tokens, Token{Type: TokNumber, Val: input[start:i]})
			continue
		}

		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_' {
			start := i
			for i < n && ((input[i] >= 'a' && input[i] <= 'z') || (input[i] >= 'A' && input[i] <= 'Z') || (input[i] >= '0' && input[i] <= '9') || input[i] == '_') {
				i++
			}
			ident := input[start:i]
			if strings.EqualFold(ident, "Mod") {
				tokens = append(tokens, Token{Type: TokOp, Val: "%"})
			} else {
				tokens = append(tokens, Token{Type: TokIdent, Val: ident})
			}
			continue
		}

		if ch == '=' {
			if i+1 < n && input[i+1] == '=' {
				tokens = append(tokens, Token{Type: TokOp, Val: "=="})
				i += 2
			} else {
				tokens = append(tokens, Token{Type: TokOp, Val: "="})
				i++
			}
			continue
		}

		if ch == '!' {
			if i+1 < n && input[i+1] == '=' {
				tokens = append(tokens, Token{Type: TokOp, Val: "!="})
				i += 2
			} else {
				return nil, fmt.Errorf("unexpected character '!': expected '!='")
			}
			continue
		}

		if ch == '<' {
			if i+1 < n && input[i+1] == '=' {
				tokens = append(tokens, Token{Type: TokOp, Val: "<="})
				i += 2
			} else if i+1 < n && input[i+1] == '>' {
				tokens = append(tokens, Token{Type: TokOp, Val: "<>"})
				i += 2
			} else {
				tokens = append(tokens, Token{Type: TokOp, Val: "<"})
				i++
			}
			continue
		}

		if ch == '>' {
			if i+1 < n && input[i+1] == '=' {
				tokens = append(tokens, Token{Type: TokOp, Val: ">="})
				i += 2
			} else {
				tokens = append(tokens, Token{Type: TokOp, Val: ">"})
				i++
			}
			continue
		}

		if ch == '+' || ch == '-' || ch == '*' || ch == '/' || ch == '%' {
			tokens = append(tokens, Token{Type: TokOp, Val: string(ch)})
			i++
			continue
		}

		if ch == '(' {
			tokens = append(tokens, Token{Type: TokLParen, Val: "("})
			i++
			continue
		}

		if ch == ')' {
			tokens = append(tokens, Token{Type: TokRParen, Val: ")"})
			i++
			continue
		}

		return nil, fmt.Errorf("unexpected character in expression: %c", ch)
	}

	tokens = append(tokens, Token{Type: TokEOF, Val: ""})
	return tokens, nil
}

type exprParser struct {
	tokens []Token
	pos    int
}

func parseExpr(input string) (ExprNode, error) {
	tokens, err := tokenizeExpr(input)
	if err != nil {
		return nil, err
	}
	p := &exprParser{tokens: tokens, pos: 0}
	node, err := p.parseComparison()
	if err != nil {
		return nil, err
	}
	if p.tokens[p.pos].Type != TokEOF {
		return nil, fmt.Errorf("unexpected token at end of expression: %s", p.tokens[p.pos].Val)
	}
	return node, nil
}

func (p *exprParser) parseComparison() (ExprNode, error) {
	left, err := p.parseAddition()
	if err != nil {
		return nil, err
	}

	for p.pos < len(p.tokens) {
		tok := p.tokens[p.pos]
		if tok.Type == TokOp && isComparisonOp(tok.Val) {
			p.pos++
			right, err := p.parseAddition()
			if err != nil {
				return nil, err
			}
			left = &BinaryNode{Left: left, Op: tok.Val, Right: right}
		} else {
			break
		}
	}
	return left, nil
}

func (p *exprParser) parseAddition() (ExprNode, error) {
	left, err := p.parseMultiplication()
	if err != nil {
		return nil, err
	}

	for p.pos < len(p.tokens) {
		tok := p.tokens[p.pos]
		if tok.Type == TokOp && (tok.Val == "+" || tok.Val == "-") {
			p.pos++
			right, err := p.parseMultiplication()
			if err != nil {
				return nil, err
			}
			left = &BinaryNode{Left: left, Op: tok.Val, Right: right}
		} else {
			break
		}
	}
	return left, nil
}

func (p *exprParser) parseMultiplication() (ExprNode, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}

	for p.pos < len(p.tokens) {
		tok := p.tokens[p.pos]
		if tok.Type == TokOp && (tok.Val == "*" || tok.Val == "/" || tok.Val == "%") {
			p.pos++
			right, err := p.parsePrimary()
			if err != nil {
				return nil, err
			}
			left = &BinaryNode{Left: left, Op: tok.Val, Right: right}
		} else {
			break
		}
	}
	return left, nil
}

func (p *exprParser) parsePrimary() (ExprNode, error) {
	if p.pos >= len(p.tokens) {
		return nil, fmt.Errorf("unexpected end of expression")
	}

	tok := p.tokens[p.pos]
	p.pos++

	switch tok.Type {
	case TokNumber:
		isFloat := strings.Contains(tok.Val, ".")
		return &NumberNode{Value: tok.Val, IsFloat: isFloat}, nil
	case TokString:
		return &StringNode{Value: tok.Val}, nil
	case TokIdent:
		return &VarNode{Name: tok.Val}, nil
	case TokLParen:
		expr, err := p.parseComparison()
		if err != nil {
			return nil, err
		}
		if p.pos >= len(p.tokens) || p.tokens[p.pos].Type != TokRParen {
			return nil, fmt.Errorf("expected closing parenthesis ')'")
		}
		p.pos++
		return expr, nil
	default:
		return nil, fmt.Errorf("unexpected token in expression: %s", tok.Val)
	}
}


func parseMsgBoxArgs(rawArgs string) ([]string, error) {
	trimmed := strings.TrimSpace(rawArgs)
	if trimmed == "" {
		return nil, fmt.Errorf("MsgBox requires at least 1 argument")
	}

	// First, if trimmed starts with "(" and ends with ")", check if the outer parentheses wrap the ENTIRE argument list.
	if strings.HasPrefix(trimmed, "(") && strings.HasSuffix(trimmed, ")") {
		depth := 0
		inString := false
		enclosed := true
		for i := 0; i < len(trimmed); i++ {
			ch := trimmed[i]
			if ch == '"' {
				inString = !inString
			} else if !inString {
				if ch == '(' {
					depth++
				} else if ch == ')' {
					depth--
					if depth == 0 && i < len(trimmed)-1 {
						enclosed = false
						break
					}
				}
			}
		}
		if enclosed && depth == 0 {
			trimmed = strings.TrimSpace(trimmed[1 : len(trimmed)-1])
		}
	}

	if trimmed == "" {
		return nil, fmt.Errorf("MsgBox requires at least 1 argument")
	}

	var args []string
	var current strings.Builder
	depth := 0
	inString := false

	for i := 0; i < len(trimmed); i++ {
		ch := trimmed[i]
		if ch == '"' {
			inString = !inString
			current.WriteByte(ch)
		} else if inString {
			current.WriteByte(ch)
		} else {
			if ch == '(' {
				depth++
				current.WriteByte(ch)
			} else if ch == ')' {
				depth--
				current.WriteByte(ch)
			} else if ch == ',' && depth == 0 {
				args = append(args, strings.TrimSpace(current.String()))
				current.Reset()
			} else {
				current.WriteByte(ch)
			}
		}
	}
	if current.Len() > 0 {
		args = append(args, strings.TrimSpace(current.String()))
	}

	if len(args) == 0 || len(args) > 2 {
		return nil, fmt.Errorf("MsgBox requires 1 or 2 arguments, got %d", len(args))
	}

	return args, nil
}

// Transpile converts a .vbx file content into standard C code.
type blockKind int

const (
	blockIf blockKind = iota
	blockElse
	blockFor
)

type blockInfo struct {
	kind   blockKind
	forVar string
}

// Transpile converts a .vbx file content into standard C code.
func Transpile(vbxPath string) (string, error) {
	if _, err := os.Stat(vbxPath); os.IsNotExist(err) {
		return "", fmt.Errorf("source file does not exist: %s", vbxPath)
	} else if err != nil {
		return "", fmt.Errorf("error accessing file %s: %w", vbxPath, err)
	}

	file, err := os.Open(vbxPath)
	if err != nil {
		return "", fmt.Errorf("failed to open file %s: %w", vbxPath, err)
	}
	defer file.Close()

	var statements []string
	symbolTable := make(map[string]DataType)
	var blockStack []blockInfo

	needsStdio := false
	needsStdlib := false
	needsString := false
	needsConcatHelper := false
	needsMsgBox := false

	dimRegex := regexp.MustCompile(`(?i)^\s*Dim\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*=\s*(.+)$`)
	assignRegex := regexp.MustCompile(`(?i)^\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*=\s*(.+)$`)
	printQuoteRegex := regexp.MustCompile(`(?i)^\s*Print\s+"(.*)"\s*$`)
	printParenQuoteRegex := regexp.MustCompile(`(?i)^\s*Print\s*\(\s*"(.*)"\s*\)\s*$`)
	printExprRegex := regexp.MustCompile(`(?i)^\s*Print\s+(.+)$`)
	printParenExprRegex := regexp.MustCompile(`(?i)^\s*Print\s*\(\s*(.+)\s*\)\s*$`)

	ifRegex := regexp.MustCompile(`(?i)^\s*If\s+(.+)\s+Then\s*$`)
	elseRegex := regexp.MustCompile(`(?i)^\s*Else\s*$`)
	endIfRegex := regexp.MustCompile(`(?i)^\s*End\s+If\s*$`)
	forRegex := regexp.MustCompile(`(?i)^\s*For\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*=\s*(.+)\s+To\s+(.+)\s*$`)
	nextRegex := regexp.MustCompile(`(?i)^\s*Next(?:\s+([a-zA-Z_][a-zA-Z0-9_]*))?\s*$`)
	msgboxRegex := regexp.MustCompile(`(?i)^\s*MsgBox\b(.*)$`)

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "'") {
			continue
		}

		indent := strings.Repeat("    ", len(blockStack)+1)

		if matches := ifRegex.FindStringSubmatch(line); len(matches) > 1 {
			condStr := matches[1]
			condNode, err := parseExpr(condStr)
			if err != nil {
				return "", fmt.Errorf("syntax error on line %d: invalid condition in If statement: %w", lineNum, err)
			}
			_, err = condNode.ExprType(symbolTable)
			if err != nil {
				return "", fmt.Errorf("type error on line %d: %w", lineNum, err)
			}
			cCond, err := condNode.ToC(symbolTable)
			if err != nil {
				return "", fmt.Errorf("transpile error on line %d: %w", lineNum, err)
			}
			if strings.Contains(cCond, "vbx_concat") || strings.Contains(cCond, "vbx_") {
				needsConcatHelper = true
				needsStdio = true
				needsStdlib = true
			}
			if strings.Contains(cCond, "strcmp") {
				needsString = true
			}
			statements = append(statements, fmt.Sprintf("%sif (%s) {", indent, cCond))
			blockStack = append(blockStack, blockInfo{kind: blockIf})
		} else if elseRegex.MatchString(line) {
			if len(blockStack) == 0 || blockStack[len(blockStack)-1].kind != blockIf {
				return "", fmt.Errorf("syntax error on line %d: Else without matching If", lineNum)
			}
			blockStack[len(blockStack)-1] = blockInfo{kind: blockElse}
			outerIndent := strings.Repeat("    ", len(blockStack))
			statements = append(statements, fmt.Sprintf("%s} else {", outerIndent))
		} else if endIfRegex.MatchString(line) {
			if len(blockStack) == 0 || (blockStack[len(blockStack)-1].kind != blockIf && blockStack[len(blockStack)-1].kind != blockElse) {
				return "", fmt.Errorf("syntax error on line %d: End If without matching If", lineNum)
			}
			blockStack = blockStack[:len(blockStack)-1]
			outerIndent := strings.Repeat("    ", len(blockStack)+1)
			statements = append(statements, fmt.Sprintf("%s}", outerIndent))
		} else if matches := forRegex.FindStringSubmatch(line); len(matches) > 3 {
			varName := matches[1]
			startExprStr := matches[2]
			endExprStr := matches[3]

			startNode, err := parseExpr(startExprStr)
			if err != nil {
				return "", fmt.Errorf("syntax error on line %d: invalid start expression in For loop: %w", lineNum, err)
			}
			startDt, err := startNode.ExprType(symbolTable)
			if err != nil {
				return "", fmt.Errorf("type error on line %d: %w", lineNum, err)
			}
			if startDt != TypeInt {
				return "", fmt.Errorf("type error on line %d: For loop start expression must be integer", lineNum)
			}
			cStart, err := startNode.ToC(symbolTable)
			if err != nil {
				return "", fmt.Errorf("transpile error on line %d: %w", lineNum, err)
			}

			endNode, err := parseExpr(endExprStr)
			if err != nil {
				return "", fmt.Errorf("syntax error on line %d: invalid end expression in For loop: %w", lineNum, err)
			}
			endDt, err := endNode.ExprType(symbolTable)
			if err != nil {
				return "", fmt.Errorf("type error on line %d: %w", lineNum, err)
			}
			if endDt != TypeInt {
				return "", fmt.Errorf("type error on line %d: For loop end expression must be integer", lineNum)
			}
			cEnd, err := endNode.ToC(symbolTable)
			if err != nil {
				return "", fmt.Errorf("transpile error on line %d: %w", lineNum, err)
			}

			if strings.Contains(cStart, "vbx_") || strings.Contains(cEnd, "vbx_") {
				needsConcatHelper = true
				needsStdio = true
				needsStdlib = true
			}

			symbolTable[varName] = TypeInt
			statements = append(statements, fmt.Sprintf("%sfor (long long %s = %s; %s <= %s; %s++) {", indent, varName, cStart, varName, cEnd, varName))
			blockStack = append(blockStack, blockInfo{kind: blockFor, forVar: varName})
		} else if matches := nextRegex.FindStringSubmatch(line); matches != nil {
			if len(blockStack) == 0 || blockStack[len(blockStack)-1].kind != blockFor {
				return "", fmt.Errorf("syntax error on line %d: Next without matching For", lineNum)
			}
			topBlock := blockStack[len(blockStack)-1]
			if len(matches) > 1 && matches[1] != "" {
				if matches[1] != topBlock.forVar {
					return "", fmt.Errorf("syntax error on line %d: Next variable %s does not match For variable %s", lineNum, matches[1], topBlock.forVar)
				}
			}
			blockStack = blockStack[:len(blockStack)-1]
			outerIndent := strings.Repeat("    ", len(blockStack)+1)
			statements = append(statements, fmt.Sprintf("%s}", outerIndent))
		} else if matches := dimRegex.FindStringSubmatch(line); len(matches) > 2 {
			varName := matches[1]
			exprStr := matches[2]

			if _, exists := symbolTable[varName]; exists {
				return "", fmt.Errorf("syntax error on line %d: variable %q already declared", lineNum, varName)
			}

			exprNode, err := parseExpr(exprStr)
			if err != nil {
				return "", fmt.Errorf("syntax error on line %d: invalid expression in Dim %s: %w", lineNum, varName, err)
			}

			dt, err := exprNode.ExprType(symbolTable)
			if err != nil {
				return "", fmt.Errorf("type error on line %d: %w", lineNum, err)
			}

			cExpr, err := exprNode.ToC(symbolTable)
			if err != nil {
				return "", fmt.Errorf("transpile error on line %d: %w", lineNum, err)
			}

			if strings.Contains(cExpr, "vbx_concat") || strings.Contains(cExpr, "vbx_") {
				needsConcatHelper = true
				needsStdio = true
				needsStdlib = true
			}
			if strings.Contains(cExpr, "strcmp") {
				needsString = true
			}

			symbolTable[varName] = dt
			statements = append(statements, fmt.Sprintf("%s%s %s = %s;", indent, string(dt), varName, cExpr))
		} else if matches := msgboxRegex.FindStringSubmatch(line); len(matches) > 1 {
			rawArgs := matches[1]
			args, err := parseMsgBoxArgs(rawArgs)
			if err != nil {
				return "", fmt.Errorf("syntax error on line %d: invalid MsgBox statement: %w", lineNum, err)
			}

			msgNode, err := parseExpr(args[0])
			if err != nil {
				return "", fmt.Errorf("syntax error on line %d: invalid message expression in MsgBox: %w", lineNum, err)
			}
			msgC, err := formatStringArg(msgNode, symbolTable)
			if err != nil {
				return "", fmt.Errorf("type error on line %d: invalid message argument for MsgBox: %w", lineNum, err)
			}

			titleC := "NULL"
			if len(args) == 2 {
				titleNode, err := parseExpr(args[1])
				if err != nil {
					return "", fmt.Errorf("syntax error on line %d: invalid title expression in MsgBox: %w", lineNum, err)
				}
				titleC, err = formatStringArg(titleNode, symbolTable)
				if err != nil {
					return "", fmt.Errorf("type error on line %d: invalid title argument for MsgBox: %w", lineNum, err)
				}
			}

			if strings.Contains(msgC, "vbx_concat") || strings.Contains(msgC, "vbx_") || strings.Contains(titleC, "vbx_concat") || strings.Contains(titleC, "vbx_") {
				needsConcatHelper = true
			}
			if strings.Contains(msgC, "strcmp") || strings.Contains(titleC, "strcmp") {
				needsString = true
			}

			needsMsgBox = true
			needsStdio = true
			needsStdlib = true

			statements = append(statements, fmt.Sprintf("%svbx_msgbox(%s, %s);", indent, msgC, titleC))
		} else if matches := assignRegex.FindStringSubmatch(line); len(matches) > 2 && !strings.HasPrefix(strings.TrimSpace(line), "Print") && !strings.HasPrefix(strings.TrimSpace(line), "MsgBox") {
			varName := matches[1]
			exprStr := matches[2]

			varType, exists := symbolTable[varName]
			if !exists {
				return "", fmt.Errorf("syntax error on line %d: undefined variable %q", lineNum, varName)
			}

			exprNode, err := parseExpr(exprStr)
			if err != nil {
				return "", fmt.Errorf("syntax error on line %d: invalid expression in assignment to %s: %w", lineNum, varName, err)
			}

			dt, err := exprNode.ExprType(symbolTable)
			if err != nil {
				return "", fmt.Errorf("type error on line %d: %w", lineNum, err)
			}

			cExpr, err := exprNode.ToC(symbolTable)
			if err != nil {
				return "", fmt.Errorf("transpile error on line %d: %w", lineNum, err)
			}

			if dt != varType {
				if varType == TypeDouble && dt == TypeInt {
					// ok to assign int to double
				} else if varType != dt {
					return "", fmt.Errorf("type error on line %d: cannot assign %s to %s variable %s", lineNum, dt, varType, varName)
				}
			}

			if strings.Contains(cExpr, "vbx_concat") || strings.Contains(cExpr, "vbx_") {
				needsConcatHelper = true
				needsStdio = true
				needsStdlib = true
			}
			if strings.Contains(cExpr, "strcmp") {
				needsString = true
			}

			statements = append(statements, fmt.Sprintf("%s%s = %s;", indent, varName, cExpr))
		} else if matches := printQuoteRegex.FindStringSubmatch(line); len(matches) > 1 {
			msg := matches[1]
			statements = append(statements, fmt.Sprintf("%sprintf(\"%s\\n\");", indent, msg))
			needsStdio = true
		} else if matches := printParenQuoteRegex.FindStringSubmatch(line); len(matches) > 1 {
			msg := matches[1]
			statements = append(statements, fmt.Sprintf("%sprintf(\"%s\\n\");", indent, msg))
			needsStdio = true
		} else if matches := printParenExprRegex.FindStringSubmatch(line); len(matches) > 1 {
			exprStr := matches[1]
			exprNode, err := parseExpr(exprStr)
			if err != nil {
				return "", fmt.Errorf("syntax error on line %d: %w", lineNum, err)
			}
			dt, err := exprNode.ExprType(symbolTable)
			if err != nil {
				return "", fmt.Errorf("type error on line %d: %w", lineNum, err)
			}
			cExpr, err := exprNode.ToC(symbolTable)
			if err != nil {
				return "", fmt.Errorf("transpile error on line %d: %w", lineNum, err)
			}
			if strings.Contains(cExpr, "vbx_concat") || strings.Contains(cExpr, "vbx_") {
				needsConcatHelper = true
				needsStdlib = true
			}
			if strings.Contains(cExpr, "strcmp") {
				needsString = true
			}
			needsStdio = true

			var fmtSpec string
			switch dt {
			case TypeInt:
				fmtSpec = "%lld"
			case TypeDouble:
				fmtSpec = "%f"
			case TypeString:
				fmtSpec = "%s"
			}
			statements = append(statements, fmt.Sprintf("%sprintf(\"%s\\n\", %s);", indent, fmtSpec, cExpr))
		} else if matches := printExprRegex.FindStringSubmatch(line); len(matches) > 1 {
			exprStr := matches[1]
			exprNode, err := parseExpr(exprStr)
			if err != nil {
				return "", fmt.Errorf("syntax error on line %d: %w", lineNum, err)
			}
			dt, err := exprNode.ExprType(symbolTable)
			if err != nil {
				return "", fmt.Errorf("type error on line %d: %w", lineNum, err)
			}
			cExpr, err := exprNode.ToC(symbolTable)
			if err != nil {
				return "", fmt.Errorf("transpile error on line %d: %w", lineNum, err)
			}
			if strings.Contains(cExpr, "vbx_concat") || strings.Contains(cExpr, "vbx_") {
				needsConcatHelper = true
				needsStdlib = true
			}
			if strings.Contains(cExpr, "strcmp") {
				needsString = true
			}
			needsStdio = true

			var fmtSpec string
			switch dt {
			case TypeInt:
				fmtSpec = "%lld"
			case TypeDouble:
				fmtSpec = "%f"
			case TypeString:
				fmtSpec = "%s"
			}
			statements = append(statements, fmt.Sprintf("%sprintf(\"%s\\n\", %s);", indent, fmtSpec, cExpr))
		} else {
			return "", fmt.Errorf("syntax error on line %d: unsupported line %q", lineNum, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading file %s: %w", vbxPath, err)
	}

	if len(blockStack) > 0 {
		return "", fmt.Errorf("syntax error: unclosed control flow block at end of file")
	}

	var sb strings.Builder
	if needsMsgBox {
		sb.WriteString("#ifdef _WIN32\n#include <windows.h>\n#endif\n")
	}
	if needsStdio {
		sb.WriteString("#include <stdio.h>\n")
	}
	if needsStdlib || needsConcatHelper || needsMsgBox {
		sb.WriteString("#include <stdlib.h>\n")
	}
	if needsString || needsConcatHelper || needsMsgBox {
		sb.WriteString("#include <string.h>\n")
	}
	if needsStdio || needsStdlib || needsString || needsConcatHelper || needsMsgBox {
		sb.WriteString("\n")
	}

	if needsMsgBox {
		sb.WriteString(`#ifdef _WIN32
static void vbx_msgbox(const char* message, const char* title) {
    MessageBoxA(NULL, message, title ? title : "VBX", MB_OK | MB_ICONINFORMATION);
}
#elif defined(__APPLE__)
static void vbx_msgbox(const char* message, const char* title) {
    const char* t = title ? title : "VBX";
    char cmd[1024];
    snprintf(cmd, sizeof(cmd), "osascript -e 'display dialog \"%s\" with title \"%s\" buttons {\"OK\"} default button \"OK\"' >/dev/null 2>&1", message, t);
    system(cmd);
}
#else
static void vbx_msgbox(const char* message, const char* title) {
    const char* t = title ? title : "VBX";
    char cmd[1024];
    snprintf(cmd, sizeof(cmd), "zenity --info --title=\"%s\" --text=\"%s\" 2>/dev/null", t, message);
    int ret = system(cmd);
    if (ret != 0) {
        printf("+--------------------------------------------------+\n");
        printf("| %-48s |\n", t);
        printf("+--------------------------------------------------+\n");
        printf("| %-48s |\n", message);
        printf("+--------------------------------------------------+\n");
    }
}
#endif

`)
	}

	if needsConcatHelper {
		sb.WriteString(`static char* vbx_concat(const char* s1, const char* s2) {
    size_t len1 = strlen(s1);
    size_t len2 = strlen(s2);
    char* result = (char*)malloc(len1 + len2 + 1);
    if (!result) return "";
    memcpy(result, s1, len1);
    memcpy(result + len1, s2, len2 + 1);
    return result;
}

static char* vbx_int_to_str(long long n) {
    char* buf = (char*)malloc(32);
    if (!buf) return "";
    snprintf(buf, 32, "%lld", n);
    return buf;
}

static char* vbx_double_to_str(double d) {
    char* buf = (char*)malloc(64);
    if (!buf) return "";
    snprintf(buf, 64, "%f", d);
    return buf;
}

`)
	}

	sb.WriteString("int main(void) {\n")
	for _, stmt := range statements {
		sb.WriteString(stmt)
		sb.WriteString("\n")
	}
	sb.WriteString("    return 0;\n")
	sb.WriteString("}\n")

	return sb.String(), nil
}

// FindCCompiler looks for gcc or clang in PATH.
func FindCCompiler() (string, error) {
	if path, err := exec.LookPath("gcc"); err == nil {
		return path, nil
	}
	if path, err := exec.LookPath("clang"); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("no C compiler found: please install 'gcc' or 'clang' and ensure it is in your PATH")
}

// BuildAndRun transpiles the .vbx file, compiles the generated C code, and executes it.
func BuildAndRun(vbxPath string) error {
	cCode, err := Transpile(vbxPath)
	if err != nil {
		return err
	}

	compiler, err := FindCCompiler()
	if err != nil {
		return err
	}

	buildDir := "build"
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return fmt.Errorf("failed to create build directory: %w", err)
	}

	cFilePath := filepath.Join(buildDir, "temp.c")
	if err := os.WriteFile(cFilePath, []byte(cCode), 0644); err != nil {
		return fmt.Errorf("failed to write C source file: %w", err)
	}

	execExt := ""
	if runtime.GOOS == "windows" {
		execExt = ".exe"
	}
	execPath := filepath.Join(buildDir, "temp"+execExt)

	cmdCompile := exec.Command(compiler, cFilePath, "-o", execPath)
	cmdCompile.Stdout = os.Stdout
	cmdCompile.Stderr = os.Stderr
	if err := cmdCompile.Run(); err != nil {
		return fmt.Errorf("C compilation failed: %w", err)
	}

	cmdRun := exec.Command(execPath)
	cmdRun.Stdout = os.Stdout
	cmdRun.Stderr = os.Stderr
	cmdRun.Stdin = os.Stdin
	if err := cmdRun.Run(); err != nil {
		return fmt.Errorf("execution failed: %w", err)
	}

	return nil
}
