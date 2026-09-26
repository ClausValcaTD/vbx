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
