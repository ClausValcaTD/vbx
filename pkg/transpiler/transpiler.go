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
	needsStdio := false

	// Regex patterns for Print "msg" and Print("msg")
	// Matches: Print "..." or Print("...") with optional whitespace
	printQuoteRegex := regexp.MustCompile(`^\s*Print\s+"(.*)"\s*$`)
	printParenRegex := regexp.MustCompile(`^\s*Print\s*\(\s*"(.*)"\s*\)\s*$`)

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if matches := printQuoteRegex.FindStringSubmatch(line); len(matches) > 1 {
			msg := matches[1]
			statements = append(statements, fmt.Sprintf("    printf(\"%s\\n\");", msg))
			needsStdio = true
		} else if matches := printParenRegex.FindStringSubmatch(line); len(matches) > 1 {
			msg := matches[1]
			statements = append(statements, fmt.Sprintf("    printf(\"%s\\n\");", msg))
			needsStdio = true
		} else {
			return "", fmt.Errorf("syntax error on line %d: unsupported line %q", lineNum, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading file %s: %w", vbxPath, err)
	}

	var sb strings.Builder
	if needsStdio {
		sb.WriteString("#include <stdio.h>\n\n")
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
