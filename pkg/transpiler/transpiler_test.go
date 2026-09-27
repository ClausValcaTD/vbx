package transpiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTranspilePrintQuote(t *testing.T) {
	tmpDir := t.TempDir()
	vbxFile := filepath.Join(tmpDir, "test_quote.vbx")
	content := []byte(`Print "Hello World"`)
	if err := os.WriteFile(vbxFile, content, 0644); err != nil {
		t.Fatalf("Failed to write temp vbx file: %v", err)
	}

	cCode, err := Transpile(vbxFile)
	if err != nil {
		t.Fatalf("Transpile failed: %v", err)
	}

	if !strings.Contains(cCode, "#include <stdio.h>") {
		t.Errorf("Expected #include <stdio.h>, got:\n%s", cCode)
	}

	if !strings.Contains(cCode, `printf("Hello World\n");`) {
		t.Errorf("Expected printf call, got:\n%s", cCode)
	}
}

func TestTranspilePrintParen(t *testing.T) {
	tmpDir := t.TempDir()
	vbxFile := filepath.Join(tmpDir, "test_paren.vbx")
	content := []byte(`Print("Hello Parentheses")`)
	if err := os.WriteFile(vbxFile, content, 0644); err != nil {
		t.Fatalf("Failed to write temp vbx file: %v", err)
	}

	cCode, err := Transpile(vbxFile)
	if err != nil {
		t.Fatalf("Transpile failed: %v", err)
	}

	if !strings.Contains(cCode, `printf("Hello Parentheses\n");`) {
		t.Errorf("Expected printf call, got:\n%s", cCode)
	}
}

func TestTranspileVariablesAndMath(t *testing.T) {
	tmpDir := t.TempDir()
	vbxFile := filepath.Join(tmpDir, "test_vars.vbx")
	content := []byte(`
Dim x = 10
Dim pi = 3.14
Dim name = "Visual Basic X"
x = x + 5
Dim total = x * 2
Print x
Print pi
Print name
Print total
Print "Result: " + x
`)
	if err := os.WriteFile(vbxFile, content, 0644); err != nil {
		t.Fatalf("Failed to write temp vbx file: %v", err)
	}

	cCode, err := Transpile(vbxFile)
	if err != nil {
		t.Fatalf("Transpile failed: %v", err)
	}

	expectedSnippets := []string{
		"long long x = 10LL;",
		"double pi = 3.14;",
		`const char* name = "Visual Basic X";`,
		"x = (x + 5LL);",
		"long long total = (x * 2LL);",
		`printf("%lld\n", x);`,
		`printf("%f\n", pi);`,
		`printf("%s\n", name);`,
		`printf("%lld\n", total);`,
		`printf("%s\n", vbx_concat("Result: ", vbx_int_to_str(x)));`,
	}

	for _, snippet := range expectedSnippets {
		if !strings.Contains(cCode, snippet) {
			t.Errorf("Expected snippet %q in C code, but not found.\nGenerated C code:\n%s", snippet, cCode)
		}
	}
}

func TestTranspileUndefinedVariable(t *testing.T) {
	tmpDir := t.TempDir()
	vbxFile := filepath.Join(tmpDir, "undef.vbx")
	content := []byte(`Print x`)
	if err := os.WriteFile(vbxFile, content, 0644); err != nil {
		t.Fatalf("Failed to write temp vbx file: %v", err)
	}

	_, err := Transpile(vbxFile)
	if err == nil {
		t.Fatalf("Expected error for undefined variable, got nil")
	}
}

func TestTranspileDuplicateDeclaration(t *testing.T) {
	tmpDir := t.TempDir()
	vbxFile := filepath.Join(tmpDir, "redecl.vbx")
	content := []byte("Dim x = 1\nDim x = 2")
	if err := os.WriteFile(vbxFile, content, 0644); err != nil {
		t.Fatalf("Failed to write temp vbx file: %v", err)
	}

	_, err := Transpile(vbxFile)
	if err == nil {
		t.Fatalf("Expected error for duplicate declaration, got nil")
	}
}

func TestTranspileFileNotFound(t *testing.T) {
	_, err := Transpile("non_existent_file.vbx")
	if err == nil {
		t.Fatalf("Expected error for non-existent file, got nil")
	}
}

func TestTranspileSyntaxError(t *testing.T) {
	tmpDir := t.TempDir()
	vbxFile := filepath.Join(tmpDir, "invalid.vbx")
	content := []byte(`InvalidSyntax`)
	if err := os.WriteFile(vbxFile, content, 0644); err != nil {
		t.Fatalf("Failed to write temp vbx file: %v", err)
	}

	_, err := Transpile(vbxFile)
	if err == nil {
		t.Fatalf("Expected error for syntax error, got nil")
	}
}

func TestBuildAndRunVariables(t *testing.T) {
	tmpDir := t.TempDir()
	vbxFile := filepath.Join(tmpDir, "run_vars.vbx")
	content := []byte("Dim x = 5\nx = x + 10\nPrint x")
	if err := os.WriteFile(vbxFile, content, 0644); err != nil {
		t.Fatalf("Failed to write temp vbx file: %v", err)
	}

	err := BuildAndRun(vbxFile)
	if err != nil {
		t.Fatalf("BuildAndRun failed: %v", err)
	}
}

func TestTranspileControlFlow(t *testing.T) {
	tmpDir := t.TempDir()
	vbxFile := filepath.Join(tmpDir, "test_control_flow.vbx")
	content := []byte(`
For i = 1 To 3
    If i % 2 == 0 Then
        Print i
    Else
        Print i + 10
    End If
Next i
`)
	if err := os.WriteFile(vbxFile, content, 0644); err != nil {
		t.Fatalf("Failed to write temp vbx file: %v", err)
	}

	cCode, err := Transpile(vbxFile)
	if err != nil {
		t.Fatalf("Transpile failed: %v", err)
	}

	expectedSnippets := []string{
		"for (long long i = 1LL; i <= 3LL; i++) {",
		"if (((i % 2LL) == 0LL)) {",
		"printf(\"%lld\\n\", i);",
		"} else {",
		"printf(\"%lld\\n\", (i + 10LL));",
		"}",
	}

	for _, snippet := range expectedSnippets {
		if !strings.Contains(cCode, snippet) {
			t.Errorf("Expected snippet %q in C code, but not found.\nGenerated C code:\n%s", snippet, cCode)
		}
	}
}

func TestTranspileControlFlowErrors(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "Else without If",
			content: "Else\nPrint 1\nEnd If",
		},
		{
			name:    "End If without If",
			content: "End If",
		},
		{
			name:    "Next without For",
			content: "Next i",
		},
		{
			name:    "Mismatched Next Variable",
			content: "For i = 1 To 5\nNext j",
		},
		{
			name:    "Unclosed If block",
			content: "If 1 == 1 Then\nPrint 1",
		},
		{
			name:    "Unclosed For block",
			content: "For i = 1 To 5\nPrint i",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			vbxFile := filepath.Join(tmpDir, "err.vbx")
			if err := os.WriteFile(vbxFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to write temp vbx file: %v", err)
			}

			_, err := Transpile(vbxFile)
			if err == nil {
				t.Errorf("Expected transpile error for %q, got nil", tt.name)
			}
		})
	}
}

func TestBuildAndRunControlFlow(t *testing.T) {
	tmpDir := t.TempDir()
	vbxFile := filepath.Join(tmpDir, "run_control_flow.vbx")
	content := []byte(`
Dim sum = 0
For i = 1 To 5
    If i > 2 Then
        sum = sum + i
    End If
Next i
Print sum
`)
	if err := os.WriteFile(vbxFile, content, 0644); err != nil {
		t.Fatalf("Failed to write temp vbx file: %v", err)
	}

	err := BuildAndRun(vbxFile)
	if err != nil {
		t.Fatalf("BuildAndRun failed: %v", err)
	}
}
