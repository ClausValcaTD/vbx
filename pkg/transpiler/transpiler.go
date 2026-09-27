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
	TypeVoid    DataType = "void"
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
	return "\"" + s.Value + "\"", nil
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

type ParamInfo struct {
	Name string
	Type DataType
}

type FunctionInfo struct {
	Name       string
	IsSub      bool
	Params     []ParamInfo
	ReturnType DataType
	BodyLines  []LineInfo
}

type LineInfo struct {
	LineNum int
	Text    string
}

type CallNode struct {
	Name string
	Args []ExprNode
}

func (c *CallNode) ExprType(env map[string]DataType) (DataType, error) {
	if c.Name == "InputBox" || c.Name == "File.Read" {
		return TypeString, nil
	}
	t, ok := env[c.Name]
	if !ok {
		return TypeUnknown, fmt.Errorf("undefined variable or function: %s", c.Name)
	}
	if t == TypeVoid {
		return TypeUnknown, fmt.Errorf("subroutine %s does not return a value", c.Name)
	}
	return t, nil
}

func (c *CallNode) ToC(env map[string]DataType) (string, error) {
	if c.Name == "InputBox" {
		if len(c.Args) < 1 || len(c.Args) > 2 {
			return "", fmt.Errorf("InputBox requires 1 or 2 arguments, got %d", len(c.Args))
		}
		arg0C, err := formatStringArg(c.Args[0], env)
		if err != nil {
			return "", err
		}
		if len(c.Args) == 1 {
			return fmt.Sprintf("vbx_inputbox(%s, NULL)", arg0C), nil
		}
		arg1C, err := formatStringArg(c.Args[1], env)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("vbx_inputbox(%s, %s)", arg0C, arg1C), nil
	}
	if c.Name == "File.Read" {
		if len(c.Args) != 1 {
			return "", fmt.Errorf("File.Read requires 1 argument, got %d", len(c.Args))
		}
		arg0C, err := formatStringArg(c.Args[0], env)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("vbx_file_read(%s)", arg0C), nil
	}
	t, ok := env[c.Name]
	if !ok {
		return "", fmt.Errorf("undefined function: %s", c.Name)
	}
	if t == TypeVoid {
		return "", fmt.Errorf("subroutine %s does not return a value", c.Name)
	}
	var cArgs []string
	for _, arg := range c.Args {
		argC, err := arg.ToC(env)
		if err != nil {
			return "", err
		}
		cArgs = append(cArgs, argC)
	}
	return fmt.Sprintf("%s(%s)", c.Name, strings.Join(cArgs, ", ")), nil
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

type TokenType int

const (
	TokNumber TokenType = iota
	TokString
	TokIdent
	TokOp
	TokLParen
	TokRParen
	TokComma
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
			i++
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
			for i < n && ((input[i] >= 'a' && input[i] <= 'z') || (input[i] >= 'A' && input[i] <= 'Z') || (input[i] >= '0' && input[i] <= '9') || input[i] == '_' || input[i] == '.') {
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

		if ch == ',' {
			tokens = append(tokens, Token{Type: TokComma, Val: ","})
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
		ident := tok.Val
		if p.pos < len(p.tokens) && p.tokens[p.pos].Type == TokLParen {
			p.pos++ // consume '('
			var args []ExprNode
			if p.pos < len(p.tokens) && p.tokens[p.pos].Type != TokRParen {
				for {
					arg, err := p.parseComparison()
					if err != nil {
						return nil, err
					}
					args = append(args, arg)
					if p.pos < len(p.tokens) && p.tokens[p.pos].Type == TokComma {
						p.pos++ // consume ','
						continue
					}
					break
				}
			}
			if p.pos >= len(p.tokens) || p.tokens[p.pos].Type != TokRParen {
				return nil, fmt.Errorf("expected closing parenthesis ')' in call to %s", ident)
			}
			p.pos++ // consume ')'
			return &CallNode{Name: ident, Args: args}, nil
		}
		return &VarNode{Name: ident}, nil
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

type blockKind int

const (
	blockIf blockKind = iota
	blockElse
	blockFor
	blockSub
	blockFunction
)

type blockInfo struct {
	kind   blockKind
	forVar string
}

func splitCommaArgs(raw string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
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
	if inString {
		return nil, fmt.Errorf("unterminated string literal in argument list")
	}
	if depth != 0 {
		return nil, fmt.Errorf("unmatched parentheses in argument list")
	}
	if current.Len() > 0 {
		args = append(args, strings.TrimSpace(current.String()))
	}
	return args, nil
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

	var lines []LineInfo
	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "'") {
			continue
		}
		lines = append(lines, LineInfo{LineNum: lineNum, Text: line})
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading file %s: %w", vbxPath, err)
	}

	subHeaderRegex := regexp.MustCompile("(?i)^\\s*Sub\\s+([a-zA-Z_][a-zA-Z0-9_]*)\\s*(?:\\((.*)\\))?\\s*$")
	funcHeaderRegex := regexp.MustCompile("(?i)^\\s*Function\\s+([a-zA-Z_][a-zA-Z0-9_]*)\\s*(?:\\((.*)\\))?\\s*$")
	endSubRegex := regexp.MustCompile("(?i)^\\s*End\\s+Sub\\s*$")
	endFuncRegex := regexp.MustCompile("(?i)^\\s*End\\s+Function\\s*$")
	returnRegex := regexp.MustCompile("(?i)^\\s*Return(?:\\s+(.+))?\\s*$")

	dimRegex := regexp.MustCompile("(?i)^\\s*Dim\\s+([a-zA-Z_][a-zA-Z0-9_]*)\\s*=\\s*(.+)$")
	assignRegex := regexp.MustCompile("(?i)^\\s*([a-zA-Z_][a-zA-Z0-9_]*)\\s*=\\s*(.+)$")
	ifRegex := regexp.MustCompile("(?i)^\\s*If\\s+(.+)\\s+Then\\s*$")
	elseRegex := regexp.MustCompile("(?i)^\\s*Else\\s*$")
	endIfRegex := regexp.MustCompile("(?i)^\\s*End\\s+If\\s*$")
	forRegex := regexp.MustCompile("(?i)^\\s*For\\s+([a-zA-Z_][a-zA-Z0-9_]*)\\s*=\\s*(.+)\\s+To\\s+(.+)\\s*$")
	nextRegex := regexp.MustCompile("(?i)^\\s*Next(?:\\s+([a-zA-Z_][a-zA-Z0-9_]*))?\\s*$")
	msgboxRegex := regexp.MustCompile("(?i)^\\s*MsgBox\\b(.*)$")
	fileWriteRegex := regexp.MustCompile("(?i)^\\s*File\\.Write\\b(.*)$")
	printQuoteRegex := regexp.MustCompile("(?i)^\\s*Print\\s+\"(.*)\"\\s*$")
	printParenQuoteRegex := regexp.MustCompile("(?i)^\\s*Print\\s*\\(\\s*\"(.*)\"\\s*\\)\\s*$")
	printExprRegex := regexp.MustCompile("(?i)^\\s*Print\\s+(.+)$")
	printParenExprRegex := regexp.MustCompile("(?i)^\\s*Print\\s*\\(\\s*(.+)\\s*\\)\\s*$")

	callParenRegex := regexp.MustCompile("(?i)^\\s*([a-zA-Z_][a-zA-Z0-9_]*)\\s*\\((.*)\\)\\s*$")
	callSpaceRegex := regexp.MustCompile("(?i)^\\s*([a-zA-Z_][a-zA-Z0-9_]*)\\s+(.+)$")

	var topLevelLines []LineInfo
	var functions []*FunctionInfo
	funcMap := make(map[string]*FunctionInfo)

	var currentFunc *FunctionInfo

	for _, l := range lines {
		line := l.Text
		if matches := subHeaderRegex.FindStringSubmatch(line); len(matches) > 1 {
			if currentFunc != nil {
				return "", fmt.Errorf("syntax error on line %d: nested Sub or Function definition is not allowed", l.LineNum)
			}
			fnName := matches[1]
			rawParams := ""
			if len(matches) > 2 {
				rawParams = matches[2]
			}
			paramNames, err := splitCommaArgs(rawParams)
			if err != nil {
				return "", fmt.Errorf("syntax error on line %d in parameter list: %w", l.LineNum, err)
			}
			var params []ParamInfo
			for _, pName := range paramNames {
				pName = strings.TrimSpace(pName)
				if pName == "" {
					return "", fmt.Errorf("syntax error on line %d: empty parameter name", l.LineNum)
				}
				params = append(params, ParamInfo{Name: pName, Type: TypeInt})
			}
			fn := &FunctionInfo{
				Name:       fnName,
				IsSub:      true,
				Params:     params,
				ReturnType: TypeVoid,
			}
			functions = append(functions, fn)
			funcMap[fnName] = fn
			currentFunc = fn
			continue
		}

		if matches := funcHeaderRegex.FindStringSubmatch(line); len(matches) > 1 {
			if currentFunc != nil {
				return "", fmt.Errorf("syntax error on line %d: nested Sub or Function definition is not allowed", l.LineNum)
			}
			fnName := matches[1]
			rawParams := ""
			if len(matches) > 2 {
				rawParams = matches[2]
			}
			paramNames, err := splitCommaArgs(rawParams)
			if err != nil {
				return "", fmt.Errorf("syntax error on line %d in parameter list: %w", l.LineNum, err)
			}
			var params []ParamInfo
			for _, pName := range paramNames {
				pName = strings.TrimSpace(pName)
				if pName == "" {
					return "", fmt.Errorf("syntax error on line %d: empty parameter name", l.LineNum)
				}
				params = append(params, ParamInfo{Name: pName, Type: TypeInt})
			}
			fn := &FunctionInfo{
				Name:       fnName,
				IsSub:      false,
				Params:     params,
				ReturnType: TypeInt,
			}
			functions = append(functions, fn)
			funcMap[fnName] = fn
			currentFunc = fn
			continue
		}

		if endSubRegex.MatchString(line) {
			if currentFunc == nil || !currentFunc.IsSub {
				return "", fmt.Errorf("syntax error on line %d: End Sub without matching Sub", l.LineNum)
			}
			currentFunc = nil
			continue
		}

		if endFuncRegex.MatchString(line) {
			if currentFunc == nil || currentFunc.IsSub {
				return "", fmt.Errorf("syntax error on line %d: End Function without matching Function", l.LineNum)
			}
			currentFunc = nil
			continue
		}

		if currentFunc != nil {
			currentFunc.BodyLines = append(currentFunc.BodyLines, l)
		} else {
			topLevelLines = append(topLevelLines, l)
		}
	}

	if currentFunc != nil {
		if currentFunc.IsSub {
			return "", fmt.Errorf("syntax error: unclosed Sub block %s at end of file", currentFunc.Name)
		} else {
			return "", fmt.Errorf("syntax error: unclosed Function block %s at end of file", currentFunc.Name)
		}
	}

	globalEnv := make(map[string]DataType)
	globalEnv["InputBox"] = TypeString
	globalEnv["File.Read"] = TypeString
	for _, fn := range functions {
		globalEnv[fn.Name] = fn.ReturnType
	}

	allLines := append([]LineInfo{}, topLevelLines...)
	for _, fn := range functions {
		allLines = append(allLines, fn.BodyLines...)
	}

	inferCallTypes := func(fnName string, rawArgs []string, env map[string]DataType) {
		fn, ok := funcMap[fnName]
		if !ok {
			return
		}
		if len(rawArgs) != len(fn.Params) {
			return
		}
		for i, argStr := range rawArgs {
			exprNode, err := parseExpr(argStr)
			if err != nil {
				continue
			}
			dt, err := exprNode.ExprType(env)
			if err != nil || dt == TypeUnknown {
				continue
			}
			fn.Params[i].Type = dt
		}
	}

	for _, l := range allLines {
		line := l.Text
		if matches := dimRegex.FindStringSubmatch(line); len(matches) > 2 {
			exprStr := matches[2]
			exprNode, err := parseExpr(exprStr)
			if err == nil {
				var inspectNode func(n ExprNode)
				inspectNode = func(n ExprNode) {
					if call, ok := n.(*CallNode); ok {
						if targetFn, exists := funcMap[call.Name]; exists {
							if len(call.Args) == len(targetFn.Params) {
								for i, arg := range call.Args {
									if dt, err := arg.ExprType(globalEnv); err == nil && dt != TypeUnknown {
										targetFn.Params[i].Type = dt
									}
								}
							}
						}
					} else if bin, ok := n.(*BinaryNode); ok {
						inspectNode(bin.Left)
						inspectNode(bin.Right)
					}
				}
				inspectNode(exprNode)
			}
		} else if matches := callParenRegex.FindStringSubmatch(line); len(matches) > 2 && funcMap[matches[1]] != nil {
			fnName := matches[1]
			rawArgs := matches[2]
			args, err := splitCommaArgs(rawArgs)
			if err == nil {
				inferCallTypes(fnName, args, globalEnv)
			}
		} else if matches := callSpaceRegex.FindStringSubmatch(line); len(matches) > 2 && funcMap[matches[1]] != nil {
			fnName := matches[1]
			rawArgs := matches[2]
			args, err := splitCommaArgs(rawArgs)
			if err == nil {
				inferCallTypes(fnName, args, globalEnv)
			}
		}
	}

	for _, fn := range functions {
		if fn.IsSub {
			continue
		}
		fnEnv := make(map[string]DataType)
		for k, v := range globalEnv {
			fnEnv[k] = v
		}
		for _, p := range fn.Params {
			fnEnv[p.Name] = p.Type
		}
		for _, l := range fn.BodyLines {
			line := l.Text
			if matches := dimRegex.FindStringSubmatch(line); len(matches) > 2 {
				varName := matches[1]
				exprStr := matches[2]
				exprNode, err := parseExpr(exprStr)
				if err == nil {
					if dt, err := exprNode.ExprType(fnEnv); err == nil {
						fnEnv[varName] = dt
					}
				}
			} else if matches := returnRegex.FindStringSubmatch(line); len(matches) > 0 {
				if len(matches) > 1 && matches[1] != "" {
					retNode, err := parseExpr(matches[1])
					if err == nil {
						if dt, err := retNode.ExprType(fnEnv); err == nil {
							fn.ReturnType = dt
							globalEnv[fn.Name] = dt
						}
					}
				}
			}
		}
	}

	needsStdio := false
	needsStdlib := false
	needsString := false
	needsConcatHelper := false
	needsMsgBox := false
	needsInputBox := false
	needsFileRead := false
	needsFileWrite := false

	transpileBlock := func(bodyLines []LineInfo, localEnv map[string]DataType, isSub bool, isFunc bool) ([]string, error) {
		var stmts []string
		var blockStack []blockInfo

		var validateCalls func(n ExprNode) error
		validateCalls = func(n ExprNode) error {
			if call, ok := n.(*CallNode); ok {
				if call.Name == "InputBox" || call.Name == "File.Read" {
					if call.Name == "File.Read" && len(call.Args) != 1 {
						return fmt.Errorf("type error: File.Read expected 1 argument, got %d", len(call.Args))
					}
					if call.Name == "InputBox" && (len(call.Args) < 1 || len(call.Args) > 2) {
						return fmt.Errorf("type error: InputBox expected 1 or 2 arguments, got %d", len(call.Args))
					}
					for _, arg := range call.Args {
						if err := validateCalls(arg); err != nil {
							return err
						}
					}
					return nil
				}
				targetFn, exists := funcMap[call.Name]
				if !exists {
					return fmt.Errorf("undefined function: %s", call.Name)
				}
				if len(call.Args) != len(targetFn.Params) {
					return fmt.Errorf("type error: %s expected %d arguments, got %d", call.Name, len(targetFn.Params), len(call.Args))
				}
				for _, arg := range call.Args {
					if err := validateCalls(arg); err != nil {
						return err
					}
				}
			} else if bin, ok := n.(*BinaryNode); ok {
				if err := validateCalls(bin.Left); err != nil {
					return err
				}
				if err := validateCalls(bin.Right); err != nil {
					return err
				}
			}
			return nil
		}

		for _, l := range bodyLines {
			lineNum := l.LineNum
			line := l.Text
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "'") {
				continue
			}

			indent := strings.Repeat("    ", len(blockStack)+1)

			if matches := ifRegex.FindStringSubmatch(line); len(matches) > 1 {
				condStr := matches[1]
				condNode, err := parseExpr(condStr)
				if err != nil {
					return nil, fmt.Errorf("syntax error on line %d: invalid condition in If statement: %w", lineNum, err)
				}
				_, err = condNode.ExprType(localEnv)
				if err != nil {
					return nil, fmt.Errorf("type error on line %d: %w", lineNum, err)
				}
				if err := validateCalls(condNode); err != nil {
					return nil, fmt.Errorf("type error on line %d: %w", lineNum, err)
				}
				cCond, err := condNode.ToC(localEnv)
				if err != nil {
					return nil, fmt.Errorf("transpile error on line %d: %w", lineNum, err)
				}
				if strings.Contains(cCond, "vbx_concat") || strings.Contains(cCond, "vbx_") {
					needsConcatHelper = true
					needsStdio = true
					needsStdlib = true
				}
				if strings.Contains(cCond, "strcmp") {
					needsString = true
				}
				stmts = append(stmts, fmt.Sprintf("%sif (%s) {", indent, cCond))
				blockStack = append(blockStack, blockInfo{kind: blockIf})
			} else if elseRegex.MatchString(line) {
				if len(blockStack) == 0 || blockStack[len(blockStack)-1].kind != blockIf {
					return nil, fmt.Errorf("syntax error on line %d: Else without matching If", lineNum)
				}
				blockStack[len(blockStack)-1] = blockInfo{kind: blockElse}
				outerIndent := strings.Repeat("    ", len(blockStack))
				stmts = append(stmts, fmt.Sprintf("%s} else {", outerIndent))
			} else if endIfRegex.MatchString(line) {
				if len(blockStack) == 0 || (blockStack[len(blockStack)-1].kind != blockIf && blockStack[len(blockStack)-1].kind != blockElse) {
					return nil, fmt.Errorf("syntax error on line %d: End If without matching If", lineNum)
				}
				blockStack = blockStack[:len(blockStack)-1]
				outerIndent := strings.Repeat("    ", len(blockStack)+1)
				stmts = append(stmts, fmt.Sprintf("%s}", outerIndent))
			} else if matches := forRegex.FindStringSubmatch(line); len(matches) > 3 {
				varName := matches[1]
				startExprStr := matches[2]
				endExprStr := matches[3]

				startNode, err := parseExpr(startExprStr)
				if err != nil {
					return nil, fmt.Errorf("syntax error on line %d: invalid start expression in For loop: %w", lineNum, err)
				}
				startDt, err := startNode.ExprType(localEnv)
				if err != nil {
					return nil, fmt.Errorf("type error on line %d: %w", lineNum, err)
				}
				if startDt != TypeInt {
					return nil, fmt.Errorf("type error on line %d: For loop start expression must be integer", lineNum)
				}
				cStart, err := startNode.ToC(localEnv)
				if err != nil {
					return nil, fmt.Errorf("transpile error on line %d: %w", lineNum, err)
				}

				endNode, err := parseExpr(endExprStr)
				if err != nil {
					return nil, fmt.Errorf("syntax error on line %d: invalid end expression in For loop: %w", lineNum, err)
				}
				endDt, err := endNode.ExprType(localEnv)
				if err != nil {
					return nil, fmt.Errorf("type error on line %d: %w", lineNum, err)
				}
				if endDt != TypeInt {
					return nil, fmt.Errorf("type error on line %d: For loop end expression must be integer", lineNum)
				}
				cEnd, err := endNode.ToC(localEnv)
				if err != nil {
					return nil, fmt.Errorf("transpile error on line %d: %w", lineNum, err)
				}

				if strings.Contains(cStart, "vbx_") || strings.Contains(cEnd, "vbx_") {
					needsConcatHelper = true
					needsStdio = true
					needsStdlib = true
				}

				localEnv[varName] = TypeInt
				stmts = append(stmts, fmt.Sprintf("%sfor (long long %s = %s; %s <= %s; %s++) {", indent, varName, cStart, varName, cEnd, varName))
				blockStack = append(blockStack, blockInfo{kind: blockFor, forVar: varName})
			} else if matches := nextRegex.FindStringSubmatch(line); matches != nil {
				if len(blockStack) == 0 || blockStack[len(blockStack)-1].kind != blockFor {
					return nil, fmt.Errorf("syntax error on line %d: Next without matching For", lineNum)
				}
				topBlock := blockStack[len(blockStack)-1]
				if len(matches) > 1 && matches[1] != "" {
					if matches[1] != topBlock.forVar {
						return nil, fmt.Errorf("syntax error on line %d: Next variable %s does not match For variable %s", lineNum, matches[1], topBlock.forVar)
					}
				}
				blockStack = blockStack[:len(blockStack)-1]
				outerIndent := strings.Repeat("    ", len(blockStack)+1)
				stmts = append(stmts, fmt.Sprintf("%s}", outerIndent))
			} else if matches := returnRegex.FindStringSubmatch(line); len(matches) > 0 {
				retExprStr := ""
				if len(matches) > 1 {
					retExprStr = strings.TrimSpace(matches[1])
				}

				if isSub {
					if retExprStr != "" {
						return nil, fmt.Errorf("syntax error on line %d: Subroutine cannot return a value", lineNum)
					}
					stmts = append(stmts, fmt.Sprintf("%sreturn;", indent))
				} else if isFunc {
					if retExprStr == "" {
						return nil, fmt.Errorf("syntax error on line %d: Function Return requires an expression", lineNum)
					}
					retNode, err := parseExpr(retExprStr)
					if err != nil {
						return nil, fmt.Errorf("syntax error on line %d: invalid expression in Return: %w", lineNum, err)
					}
					_, err = retNode.ExprType(localEnv)
					if err != nil {
						return nil, fmt.Errorf("type error on line %d: %w", lineNum, err)
					}
					if err := validateCalls(retNode); err != nil {
						return nil, fmt.Errorf("type error on line %d: %w", lineNum, err)
					}
					cRet, err := retNode.ToC(localEnv)
					if err != nil {
						return nil, fmt.Errorf("transpile error on line %d: %w", lineNum, err)
					}
					if strings.Contains(cRet, "vbx_concat") || strings.Contains(cRet, "vbx_") {
						needsConcatHelper = true
						needsStdio = true
						needsStdlib = true
					}
					if strings.Contains(cRet, "strcmp") {
						needsString = true
					}
					stmts = append(stmts, fmt.Sprintf("%sreturn %s;", indent, cRet))
				} else {
					return nil, fmt.Errorf("syntax error on line %d: Return statement outside of Sub or Function", lineNum)
				}
			} else if matches := dimRegex.FindStringSubmatch(line); len(matches) > 2 {
				varName := matches[1]
				exprStr := matches[2]

				if _, exists := localEnv[varName]; exists {
					return nil, fmt.Errorf("syntax error on line %d: variable %q already declared", lineNum, varName)
				}

				exprNode, err := parseExpr(exprStr)
				if err != nil {
					return nil, fmt.Errorf("syntax error on line %d: invalid expression in Dim %s: %w", lineNum, varName, err)
				}

				dt, err := exprNode.ExprType(localEnv)
				if err != nil {
					return nil, fmt.Errorf("type error on line %d: %w", lineNum, err)
				}

				if err := validateCalls(exprNode); err != nil {
					return nil, fmt.Errorf("type error on line %d: %w", lineNum, err)
				}
				cExpr, err := exprNode.ToC(localEnv)
				if err != nil {
					return nil, fmt.Errorf("transpile error on line %d: %w", lineNum, err)
				}

				if strings.Contains(cExpr, "vbx_concat") || strings.Contains(cExpr, "vbx_") {
					needsConcatHelper = true
					needsStdio = true
					needsStdlib = true
				}
				if strings.Contains(cExpr, "strcmp") {
					needsString = true
				}

				localEnv[varName] = dt
				stmts = append(stmts, fmt.Sprintf("%s%s %s = %s;", indent, string(dt), varName, cExpr))
			} else if matches := msgboxRegex.FindStringSubmatch(line); len(matches) > 1 {
				rawArgs := matches[1]
				args, err := parseMsgBoxArgs(rawArgs)
				if err != nil {
					return nil, fmt.Errorf("syntax error on line %d: invalid MsgBox statement: %w", lineNum, err)
				}

				msgNode, err := parseExpr(args[0])
				if err != nil {
					return nil, fmt.Errorf("syntax error on line %d: invalid message expression in MsgBox: %w", lineNum, err)
				}
				msgC, err := formatStringArg(msgNode, localEnv)
				if err != nil {
					return nil, fmt.Errorf("type error on line %d: invalid message argument for MsgBox: %w", lineNum, err)
				}

				titleC := "NULL"
				if len(args) == 2 {
					titleNode, err := parseExpr(args[1])
					if err != nil {
						return nil, fmt.Errorf("syntax error on line %d: invalid title expression in MsgBox: %w", lineNum, err)
					}
					titleC, err = formatStringArg(titleNode, localEnv)
					if err != nil {
						return nil, fmt.Errorf("type error on line %d: invalid title argument for MsgBox: %w", lineNum, err)
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

				stmts = append(stmts, fmt.Sprintf("%svbx_msgbox(%s, %s);", indent, msgC, titleC))
			} else if matches := fileWriteRegex.FindStringSubmatch(line); len(matches) > 1 {
				rawArgs := matches[1]
				args, err := parseMsgBoxArgs(rawArgs)
				if err != nil || len(args) != 2 {
					return nil, fmt.Errorf("syntax error on line %d: File.Write requires 2 arguments (path, content)", lineNum)
				}

				pathNode, err := parseExpr(args[0])
				if err != nil {
					return nil, fmt.Errorf("syntax error on line %d: invalid path expression in File.Write: %w", lineNum, err)
				}
				pathC, err := formatStringArg(pathNode, localEnv)
				if err != nil {
					return nil, fmt.Errorf("type error on line %d: invalid path argument for File.Write: %w", lineNum, err)
				}

				contentNode, err := parseExpr(args[1])
				if err != nil {
					return nil, fmt.Errorf("syntax error on line %d: invalid content expression in File.Write: %w", lineNum, err)
				}
				contentC, err := formatStringArg(contentNode, localEnv)
				if err != nil {
					return nil, fmt.Errorf("type error on line %d: invalid content argument for File.Write: %w", lineNum, err)
				}

				if strings.Contains(pathC, "vbx_concat") || strings.Contains(pathC, "vbx_") || strings.Contains(contentC, "vbx_concat") || strings.Contains(contentC, "vbx_") {
					needsConcatHelper = true
				}
				if strings.Contains(pathC, "strcmp") || strings.Contains(contentC, "strcmp") {
					needsString = true
				}

				needsStdio = true

				stmts = append(stmts, fmt.Sprintf("%svbx_file_write(%s, %s);", indent, pathC, contentC))
			} else if matches := printQuoteRegex.FindStringSubmatch(line); len(matches) > 1 {
				msg := matches[1]
				stmts = append(stmts, fmt.Sprintf("%sprintf(\"%s\\n\");", indent, msg))
				needsStdio = true
			} else if matches := printParenQuoteRegex.FindStringSubmatch(line); len(matches) > 1 {
				msg := matches[1]
				stmts = append(stmts, fmt.Sprintf("%sprintf(\"%s\\n\");", indent, msg))
				needsStdio = true
			} else if matches := printParenExprRegex.FindStringSubmatch(line); len(matches) > 1 {
				exprStr := matches[1]
				exprNode, err := parseExpr(exprStr)
				if err != nil {
					return nil, fmt.Errorf("syntax error on line %d: %w", lineNum, err)
				}
				dt, err := exprNode.ExprType(localEnv)
				if err != nil {
					return nil, fmt.Errorf("type error on line %d: %w", lineNum, err)
				}
				if err := validateCalls(exprNode); err != nil {
					return nil, fmt.Errorf("type error on line %d: %w", lineNum, err)
				}
				cExpr, err := exprNode.ToC(localEnv)
				if err != nil {
					return nil, fmt.Errorf("transpile error on line %d: %w", lineNum, err)
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
				stmts = append(stmts, fmt.Sprintf("%sprintf(\"%s\\n\", %s);", indent, fmtSpec, cExpr))
			} else if matches := printExprRegex.FindStringSubmatch(line); len(matches) > 1 {
				exprStr := matches[1]
				exprNode, err := parseExpr(exprStr)
				if err != nil {
					return nil, fmt.Errorf("syntax error on line %d: %w", lineNum, err)
				}
				dt, err := exprNode.ExprType(localEnv)
				if err != nil {
					return nil, fmt.Errorf("type error on line %d: %w", lineNum, err)
				}
				if err := validateCalls(exprNode); err != nil {
					return nil, fmt.Errorf("type error on line %d: %w", lineNum, err)
				}
				cExpr, err := exprNode.ToC(localEnv)
				if err != nil {
					return nil, fmt.Errorf("transpile error on line %d: %w", lineNum, err)
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
				stmts = append(stmts, fmt.Sprintf("%sprintf(\"%s\\n\", %s);", indent, fmtSpec, cExpr))
			} else if matches := assignRegex.FindStringSubmatch(line); len(matches) > 2 {
				varName := matches[1]
				exprStr := matches[2]

				varType, exists := localEnv[varName]
				if !exists {
					return nil, fmt.Errorf("syntax error on line %d: undefined variable %q", lineNum, varName)
				}

				exprNode, err := parseExpr(exprStr)
				if err != nil {
					return nil, fmt.Errorf("syntax error on line %d: invalid expression in assignment to %s: %w", lineNum, varName, err)
				}

				dt, err := exprNode.ExprType(localEnv)
				if err != nil {
					return nil, fmt.Errorf("type error on line %d: %w", lineNum, err)
				}

				if err := validateCalls(exprNode); err != nil {
					return nil, fmt.Errorf("type error on line %d: %w", lineNum, err)
				}
				cExpr, err := exprNode.ToC(localEnv)
				if err != nil {
					return nil, fmt.Errorf("transpile error on line %d: %w", lineNum, err)
				}

				if dt != varType {
					if varType == TypeDouble && dt == TypeInt {
						// ok to assign int to double
					} else if varType != dt {
						return nil, fmt.Errorf("type error on line %d: cannot assign %s to %s variable %s", lineNum, dt, varType, varName)
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

				stmts = append(stmts, fmt.Sprintf("%s%s = %s;", indent, varName, cExpr))
			} else if matches := callParenRegex.FindStringSubmatch(line); len(matches) > 2 && funcMap[matches[1]] != nil {
				fnName := matches[1]
				targetFn := funcMap[fnName]
				rawArgs := matches[2]
				args, err := splitCommaArgs(rawArgs)
				if err != nil {
					return nil, fmt.Errorf("syntax error on line %d in argument list: %w", lineNum, err)
				}
				if len(args) != len(targetFn.Params) {
					return nil, fmt.Errorf("type error on line %d: %s expected %d arguments, got %d", lineNum, fnName, len(targetFn.Params), len(args))
				}
				var cArgs []string
				for _, argStr := range args {
					argNode, err := parseExpr(argStr)
					if err != nil {
						return nil, fmt.Errorf("syntax error on line %d in argument: %w", lineNum, err)
					}
					cArg, err := argNode.ToC(localEnv)
					if err != nil {
						return nil, fmt.Errorf("transpile error on line %d: %w", lineNum, err)
					}
					if strings.Contains(cArg, "vbx_concat") || strings.Contains(cArg, "vbx_") {
						needsConcatHelper = true
						needsStdio = true
						needsStdlib = true
					}
					cArgs = append(cArgs, cArg)
				}
				stmts = append(stmts, fmt.Sprintf("%s%s(%s);", indent, fnName, strings.Join(cArgs, ", ")))
			} else if matches := callSpaceRegex.FindStringSubmatch(line); len(matches) > 2 && funcMap[matches[1]] != nil {
				fnName := matches[1]
				targetFn := funcMap[fnName]
				rawArgs := matches[2]
				args, err := splitCommaArgs(rawArgs)
				if err != nil {
					return nil, fmt.Errorf("syntax error on line %d in argument list: %w", lineNum, err)
				}
				if len(args) != len(targetFn.Params) {
					return nil, fmt.Errorf("type error on line %d: %s expected %d arguments, got %d", lineNum, fnName, len(targetFn.Params), len(args))
				}
				var cArgs []string
				for _, argStr := range args {
					argNode, err := parseExpr(argStr)
					if err != nil {
						return nil, fmt.Errorf("syntax error on line %d in argument: %w", lineNum, err)
					}
					cArg, err := argNode.ToC(localEnv)
					if err != nil {
						return nil, fmt.Errorf("transpile error on line %d: %w", lineNum, err)
					}
					if strings.Contains(cArg, "vbx_concat") || strings.Contains(cArg, "vbx_") {
						needsConcatHelper = true
						needsStdio = true
						needsStdlib = true
					}
					cArgs = append(cArgs, cArg)
				}
				stmts = append(stmts, fmt.Sprintf("%s%s(%s);", indent, fnName, strings.Join(cArgs, ", ")))
			} else {
				return nil, fmt.Errorf("syntax error on line %d: unsupported line %q", lineNum, line)
			}
		}

		if len(blockStack) > 0 {
			return nil, fmt.Errorf("syntax error: unclosed control flow block at end of block")
		}

		return stmts, nil
	}

	type transpiledFn struct {
		info  *FunctionInfo
		stmts []string
	}
	var transpiledFunctions []transpiledFn

	for _, fn := range functions {
		fnEnv := make(map[string]DataType)
		for k, v := range globalEnv {
			fnEnv[k] = v
		}
		for _, p := range fn.Params {
			fnEnv[p.Name] = p.Type
		}

		fnStmts, err := transpileBlock(fn.BodyLines, fnEnv, fn.IsSub, !fn.IsSub)
		if err != nil {
			return "", err
		}
		transpiledFunctions = append(transpiledFunctions, transpiledFn{
			info:  fn,
			stmts: fnStmts,
		})
	}

	mainEnv := make(map[string]DataType)
	for k, v := range globalEnv {
		mainEnv[k] = v
	}
	mainStmts, err := transpileBlock(topLevelLines, mainEnv, false, false)
	if err != nil {
		return "", err
	}

	fullBodyCode := strings.Join(mainStmts, "\n")
	for _, tFn := range transpiledFunctions {
		fullBodyCode += "\n" + strings.Join(tFn.stmts, "\n")
	}
	if strings.Contains(fullBodyCode, "vbx_inputbox") {
		needsInputBox = true
	}
	if strings.Contains(fullBodyCode, "vbx_file_read") {
		needsFileRead = true
	}
	if strings.Contains(fullBodyCode, "vbx_file_write") {
		needsFileWrite = true
	}

	var sb strings.Builder
	if needsMsgBox {
		sb.WriteString("#ifdef _WIN32\n#include <windows.h>\n#endif\n")
	}
	if needsStdio || needsInputBox || needsFileRead || needsFileWrite {
		sb.WriteString("#include <stdio.h>\n")
	}
	if needsStdlib || needsConcatHelper || needsMsgBox || needsInputBox || needsFileRead || needsFileWrite {
		sb.WriteString("#include <stdlib.h>\n")
	}
	if needsString || needsConcatHelper || needsMsgBox || needsInputBox || needsFileRead || needsFileWrite {
		sb.WriteString("#include <string.h>\n")
	}
	if needsStdio || needsStdlib || needsString || needsConcatHelper || needsMsgBox || needsInputBox || needsFileRead || needsFileWrite {
		sb.WriteString("\n")
	}

	if needsMsgBox {
		sb.WriteString("#ifdef _WIN32\n")
		sb.WriteString("static void vbx_msgbox(const char* message, const char* title) {\n")
		sb.WriteString("    MessageBoxA(NULL, message, title ? title : \"VBX\", MB_OK | MB_ICONINFORMATION);\n")
		sb.WriteString("}\n")
		sb.WriteString("#elif defined(__APPLE__)\n")
		sb.WriteString("static void vbx_msgbox(const char* message, const char* title) {\n")
		sb.WriteString("    const char* t = title ? title : \"VBX\";\n")
		sb.WriteString("    char cmd[1024];\n")
		sb.WriteString("    snprintf(cmd, sizeof(cmd), \"osascript -e 'display dialog \\\"%s\\\" with title \\\"%s\\\" buttons {\\\"OK\\\"} default button \\\"OK\\\"' >/dev/null 2>&1\", message, t);\n")
		sb.WriteString("    system(cmd);\n")
		sb.WriteString("}\n")
		sb.WriteString("#else\n")
		sb.WriteString("static void vbx_msgbox(const char* message, const char* title) {\n")
		sb.WriteString("    const char* t = title ? title : \"VBX\";\n")
		sb.WriteString("    char cmd[1024];\n")
		sb.WriteString("    snprintf(cmd, sizeof(cmd), \"zenity --info --title=\\\"%s\\\" --text=\\\"%s\\\" 2>/dev/null\", t, message);\n")
		sb.WriteString("    int ret = system(cmd);\n")
		sb.WriteString("    if (ret != 0) {\n")
		sb.WriteString("        printf(\"+--------------------------------------------------+\\n\");\n")
		sb.WriteString("        printf(\"| %-48s |\\n\", t);\n")
		sb.WriteString("        printf(\"+--------------------------------------------------+\\n\");\n")
		sb.WriteString("        printf(\"| %-48s |\\n\", message);\n")
		sb.WriteString("        printf(\"+--------------------------------------------------+\\n\");\n")
		sb.WriteString("    }\n")
		sb.WriteString("}\n")
		sb.WriteString("#endif\n\n")
	}

	if needsInputBox {
		sb.WriteString("#ifdef _WIN32\n")
		sb.WriteString("static char* vbx_inputbox(const char* prompt, const char* title) {\n")
		sb.WriteString("    const char* t = title ? title : \"VBX\";\n")
		sb.WriteString("    char cmd[2048];\n")
		sb.WriteString("    snprintf(cmd, sizeof(cmd), \"powershell -NoProfile -Command \\\"[System.Reflection.Assembly]::LoadWithPartialName('Microsoft.VisualBasic') | Out-Null; [Microsoft.VisualBasic.Interaction]::InputBox('%s', '%s')\\\"\", prompt, t);\n")
		sb.WriteString("    FILE* fp = _popen(cmd, \"r\");\n")
		sb.WriteString("    if (!fp) return \"\";\n")
		sb.WriteString("    char buf[1024];\n")
		sb.WriteString("    if (fgets(buf, sizeof(buf), fp) != NULL) {\n")
		sb.WriteString("        _pclose(fp);\n")
		sb.WriteString("        size_t len = strlen(buf);\n")
		sb.WriteString("        while (len > 0 && (buf[len-1] == '\\r' || buf[len-1] == '\\n')) { buf[--len] = '\\0'; }\n")
		sb.WriteString("        char* res = (char*)malloc(len + 1);\n")
		sb.WriteString("        if (res) strcpy(res, buf);\n")
		sb.WriteString("        return res ? res : \"\";\n")
		sb.WriteString("    }\n")
		sb.WriteString("    _pclose(fp);\n")
		sb.WriteString("    return \"\";\n")
		sb.WriteString("}\n")
		sb.WriteString("#elif defined(__APPLE__)\n")
		sb.WriteString("static char* vbx_inputbox(const char* prompt, const char* title) {\n")
		sb.WriteString("    const char* t = title ? title : \"VBX\";\n")
		sb.WriteString("    char cmd[2048];\n")
		sb.WriteString("    snprintf(cmd, sizeof(cmd), \"osascript -e 'text returned of (display dialog \\\"%s\\\" with title \\\"%s\\\" default answer \\\"\\\")' 2>/dev/null\", prompt, t);\n")
		sb.WriteString("    FILE* fp = popen(cmd, \"r\");\n")
		sb.WriteString("    if (!fp) return \"\";\n")
		sb.WriteString("    char buf[1024];\n")
		sb.WriteString("    if (fgets(buf, sizeof(buf), fp) != NULL) {\n")
		sb.WriteString("        pclose(fp);\n")
		sb.WriteString("        size_t len = strlen(buf);\n")
		sb.WriteString("        while (len > 0 && (buf[len-1] == '\\r' || buf[len-1] == '\\n')) { buf[--len] = '\\0'; }\n")
		sb.WriteString("        char* res = (char*)malloc(len + 1);\n")
		sb.WriteString("        if (res) strcpy(res, buf);\n")
		sb.WriteString("        return res ? res : \"\";\n")
		sb.WriteString("    }\n")
		sb.WriteString("    pclose(fp);\n")
		sb.WriteString("    return \"\";\n")
		sb.WriteString("}\n")
		sb.WriteString("#else\n")
		sb.WriteString("static char* vbx_inputbox(const char* prompt, const char* title) {\n")
		sb.WriteString("    const char* t = title ? title : \"VBX\";\n")
		sb.WriteString("    char cmd[2048];\n")
		sb.WriteString("    snprintf(cmd, sizeof(cmd), \"zenity --entry --title=\\\"%s\\\" --text=\\\"%s\\\" 2>/dev/null\", t, prompt);\n")
		sb.WriteString("    FILE* fp = popen(cmd, \"r\");\n")
		sb.WriteString("    char buf[1024] = {0};\n")
		sb.WriteString("    if (fp) {\n")
		sb.WriteString("        if (fgets(buf, sizeof(buf), fp) != NULL) {\n")
		sb.WriteString("            int status = pclose(fp);\n")
		sb.WriteString("            if (status == 0) {\n")
		sb.WriteString("                size_t len = strlen(buf);\n")
		sb.WriteString("                while (len > 0 && (buf[len-1] == '\\r' || buf[len-1] == '\\n')) { buf[--len] = '\\0'; }\n")
		sb.WriteString("                char* res = (char*)malloc(len + 1);\n")
		sb.WriteString("                if (res) strcpy(res, buf);\n")
		sb.WriteString("                return res ? res : \"\";\n")
		sb.WriteString("            }\n")
		sb.WriteString("        } else {\n")
		sb.WriteString("            pclose(fp);\n")
		sb.WriteString("        }\n")
		sb.WriteString("    }\n")
		sb.WriteString("    printf(\"%s: \", prompt);\n")
		sb.WriteString("    if (fgets(buf, sizeof(buf), stdin) != NULL) {\n")
		sb.WriteString("        size_t len = strlen(buf);\n")
		sb.WriteString("        while (len > 0 && (buf[len-1] == '\\r' || buf[len-1] == '\\n')) { buf[--len] = '\\0'; }\n")
		sb.WriteString("        char* res = (char*)malloc(len + 1);\n")
		sb.WriteString("        if (res) strcpy(res, buf);\n")
		sb.WriteString("        return res ? res : \"\";\n")
		sb.WriteString("    }\n")
		sb.WriteString("    return \"\";\n")
		sb.WriteString("}\n")
		sb.WriteString("#endif\n\n")
	}

	if needsFileWrite {
		sb.WriteString("static void vbx_file_write(const char* filepath, const char* content) {\n")
		sb.WriteString("    FILE* f = fopen(filepath, \"w\");\n")
		sb.WriteString("    if (!f) return;\n")
		sb.WriteString("    fputs(content ? content : \"\", f);\n")
		sb.WriteString("    fclose(f);\n")
		sb.WriteString("}\n\n")
	}

	if needsFileRead {
		sb.WriteString("static char* vbx_file_read(const char* filepath) {\n")
		sb.WriteString("    FILE* f = fopen(filepath, \"rb\");\n")
		sb.WriteString("    if (!f) return \"\";\n")
		sb.WriteString("    fseek(f, 0, SEEK_END);\n")
		sb.WriteString("    long len = ftell(f);\n")
		sb.WriteString("    if (len < 0) {\n")
		sb.WriteString("        fclose(f);\n")
		sb.WriteString("        return \"\";\n")
		sb.WriteString("    }\n")
		sb.WriteString("    fseek(f, 0, SEEK_SET);\n")
		sb.WriteString("    char* buf = (char*)malloc(len + 1);\n")
		sb.WriteString("    if (!buf) {\n")
		sb.WriteString("        fclose(f);\n")
		sb.WriteString("        return \"\";\n")
		sb.WriteString("    }\n")
		sb.WriteString("    size_t read_bytes = fread(buf, 1, len, f);\n")
		sb.WriteString("    buf[read_bytes] = '\\0';\n")
		sb.WriteString("    fclose(f);\n")
		sb.WriteString("    return buf;\n")
		sb.WriteString("}\n\n")
	}

	if needsConcatHelper {
		sb.WriteString("static char* vbx_concat(const char* s1, const char* s2) {\n")
		sb.WriteString("    size_t len1 = strlen(s1);\n")
		sb.WriteString("    size_t len2 = strlen(s2);\n")
		sb.WriteString("    char* result = (char*)malloc(len1 + len2 + 1);\n")
		sb.WriteString("    if (!result) return \"\";\n")
		sb.WriteString("    memcpy(result, s1, len1);\n")
		sb.WriteString("    memcpy(result + len1, s2, len2 + 1);\n")
		sb.WriteString("    return result;\n")
		sb.WriteString("}\n\n")
		sb.WriteString("static char* vbx_int_to_str(long long n) {\n")
		sb.WriteString("    char* buf = (char*)malloc(32);\n")
		sb.WriteString("    if (!buf) return \"\";\n")
		sb.WriteString("    snprintf(buf, 32, \"%lld\", n);\n")
		sb.WriteString("    return buf;\n")
		sb.WriteString("}\n\n")
		sb.WriteString("static char* vbx_double_to_str(double d) {\n")
		sb.WriteString("    char* buf = (char*)malloc(64);\n")
		sb.WriteString("    if (!buf) return \"\";\n")
		sb.WriteString("    snprintf(buf, 64, \"%f\", d);\n")
		sb.WriteString("    return buf;\n")
		sb.WriteString("}\n\n")
	}

	for _, fn := range functions {
		var paramSpecs []string
		for _, p := range fn.Params {
			paramSpecs = append(paramSpecs, fmt.Sprintf("%s %s", string(p.Type), p.Name))
		}
		sb.WriteString(fmt.Sprintf("%s %s(%s);\n", string(fn.ReturnType), fn.Name, strings.Join(paramSpecs, ", ")))
	}
	if len(functions) > 0 {
		sb.WriteString("\n")
	}

	for _, tFn := range transpiledFunctions {
		fn := tFn.info
		var paramSpecs []string
		for _, p := range fn.Params {
			paramSpecs = append(paramSpecs, fmt.Sprintf("%s %s", string(p.Type), p.Name))
		}
		sb.WriteString(fmt.Sprintf("%s %s(%s) {\n", string(fn.ReturnType), fn.Name, strings.Join(paramSpecs, ", ")))
		for _, stmt := range tFn.stmts {
			sb.WriteString(stmt)
			sb.WriteString("\n")
		}
		sb.WriteString("}\n\n")
	}

	sb.WriteString("int main(void) {\n")
	for _, stmt := range mainStmts {
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


// Build transpiles the .vbx file, compiles it using the C compiler with -O2, and produces an output binary executable.
func Build(vbxPath string, outputPath string, keepC bool) (string, error) {
	cCode, err := Transpile(vbxPath)
	if err != nil {
		return "", err
	}

	compiler, err := FindCCompiler()
	if err != nil {
		return "", err
	}

	var outBinaryPath string
	if outputPath != "" {
		outBinaryPath = outputPath
	} else {
		base := filepath.Base(vbxPath)
		ext := filepath.Ext(base)
		name := strings.TrimSuffix(base, ext)
		if name == "" {
			name = "app"
		}
		outBinaryPath = name
	}

	if runtime.GOOS == "windows" {
		if !strings.HasSuffix(strings.ToLower(outBinaryPath), ".exe") {
			outBinaryPath += ".exe"
		}
	}

	if dir := filepath.Dir(outBinaryPath); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	var cFilePath string
	if keepC {
		ext := filepath.Ext(outBinaryPath)
		cFilePath = strings.TrimSuffix(outBinaryPath, ext) + ".c"
		if cFilePath == outBinaryPath {
			cFilePath = outBinaryPath + ".c"
		}
	} else {
		tmpFile, err := os.CreateTemp("", "vbx_*.c")
		if err != nil {
			return "", fmt.Errorf("failed to create temporary C file: %w", err)
		}
		cFilePath = tmpFile.Name()
		tmpFile.Close()
		defer os.Remove(cFilePath)
	}

	if err := os.WriteFile(cFilePath, []byte(cCode), 0644); err != nil {
		return "", fmt.Errorf("failed to write C source file: %w", err)
	}

	cmdCompile := exec.Command(compiler, "-O2", cFilePath, "-o", outBinaryPath)
	cmdCompile.Stdout = os.Stdout
	cmdCompile.Stderr = os.Stderr
	if err := cmdCompile.Run(); err != nil {
		return "", fmt.Errorf("C compilation failed: %w", err)
	}

	return outBinaryPath, nil
}
