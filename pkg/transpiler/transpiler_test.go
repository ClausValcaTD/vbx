package transpiler

import (
	"os"
	"os/exec"
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

func TestTranspileMsgBox(t *testing.T) {
	tests := []struct {
		name             string
		content          string
		expectedSnippets []string
	}{
		{
			name:    "Basic string MsgBox statement",
			content: `MsgBox "Hello World"`,
			expectedSnippets: []string{
				"#ifdef _WIN32\n#include <windows.h>\n#endif",
				"vbx_msgbox(",
				`vbx_msgbox("Hello World", NULL);`,
				"MessageBoxA(NULL, message, title ? title : \"VBX\", MB_OK | MB_ICONINFORMATION);",
				"osascript",
				"zenity",
			},
		},
		{
			name:    "MsgBox with parens and title",
			content: `MsgBox("Operation Complete", "Success")`,
			expectedSnippets: []string{
				`vbx_msgbox("Operation Complete", "Success");`,
			},
		},
		{
			name:    "MsgBox with variables and expression concatenation",
			content: "Dim result = 42\nDim titleStr = \"Calculation Result\"\nMsgBox \"The answer is: \" + result, titleStr",
			expectedSnippets: []string{
				"long long result = 42LL;",
				`const char* titleStr = "Calculation Result";`,
				`vbx_msgbox(vbx_concat("The answer is: ", vbx_int_to_str(result)), titleStr);`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			vbxFile := filepath.Join(tmpDir, "test_msgbox.vbx")
			if err := os.WriteFile(vbxFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to write temp vbx file: %v", err)
			}

			cCode, err := Transpile(vbxFile)
			if err != nil {
				t.Fatalf("Transpile failed: %v", err)
			}

			for _, snippet := range tt.expectedSnippets {
				if !strings.Contains(cCode, snippet) {
					t.Errorf("Expected snippet %q in C code, but not found.\nGenerated C code:\n%s", snippet, cCode)
				}
			}
		})
	}
}

func TestBuildAndRunMsgBox(t *testing.T) {
	tmpDir := t.TempDir()
	vbxFile := filepath.Join(tmpDir, "run_msgbox.vbx")
	content := []byte(`
Dim x = 100
MsgBox "Value: " + x, "Test Title"
`)
	if err := os.WriteFile(vbxFile, content, 0644); err != nil {
		t.Fatalf("Failed to write temp vbx file: %v", err)
	}

	err := BuildAndRun(vbxFile)
	if err != nil {
		t.Fatalf("BuildAndRun failed: %v", err)
	}
}

func TestTranspileSubroutinesAndFunctions(t *testing.T) {
	tmpDir := t.TempDir()
	vbxFile := filepath.Join(tmpDir, "test_funcs.vbx")
	content := []byte(`
Sub Greet(name)
    Print "Hello, " + name
End Sub

Function AddNumbers(a, b)
    Return a + b
End Function

Greet "Ahmed"
Greet("Visual Basic X")
Dim total = AddNumbers(10, 20)
Print total
`)
	if err := os.WriteFile(vbxFile, content, 0644); err != nil {
		t.Fatalf("Failed to write temp vbx file: %v", err)
	}

	cCode, err := Transpile(vbxFile)
	if err != nil {
		t.Fatalf("Transpile failed: %v", err)
	}

	expectedSnippets := []string{
		"void Greet(const char* name);",
		"long long AddNumbers(long long a, long long b);",
		"void Greet(const char* name) {",
		"long long AddNumbers(long long a, long long b) {",
		"return (a + b);",
		"Greet(\"Ahmed\");",
		"Greet(\"Visual Basic X\");",
		"long long total = AddNumbers(10LL, 20LL);",
		"printf(\"%lld\\n\", total);",
	}

	for _, snippet := range expectedSnippets {
		if !strings.Contains(cCode, snippet) {
			t.Errorf("Expected snippet %q in C code, but not found.\nGenerated C code:\n%s", snippet, cCode)
		}
	}
}

func TestTranspileSubroutinesAndFunctionsErrors(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name: "Return value in Subroutine",
			content: `
Sub Greet(name)
    Return 123
End Sub
`,
		},
		{
			name: "Return without expression in Function",
			content: `
Function Add(a, b)
    Return
End Function
`,
		},
		{
			name: "Unclosed Sub block",
			content: `
Sub Greet(name)
    Print name
`,
		},
		{
			name: "Unclosed Function block",
			content: `
Function Calc(a)
    Return a * 2
`,
		},
		{
			name:    "End Sub without Sub",
			content: "End Sub",
		},
		{
			name:    "End Function without Function",
			content: "End Function",
		},
		{
			name: "Argument count mismatch",
			content: `
Function Add(a, b)
    Return a + b
End Function

Dim x = Add(10)
`,
		},
		{
			name: "Nested Sub definition",
			content: `
Sub Outer()
    Sub Inner()
    End Sub
End Sub
`,
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

func TestBuildAndRunSubroutinesAndFunctions(t *testing.T) {
	tmpDir := t.TempDir()
	vbxFile := filepath.Join(tmpDir, "run_funcs.vbx")
	content := []byte(`
Sub SayHi(name)
    Print "Hi " + name
End Sub

Function Square(n)
    Return n * n
End Function

SayHi "Alice"
Dim res = Square(6)
Print res
`)
	if err := os.WriteFile(vbxFile, content, 0644); err != nil {
		t.Fatalf("Failed to write temp vbx file: %v", err)
	}

	err := BuildAndRun(vbxFile)
	if err != nil {
		t.Fatalf("BuildAndRun failed: %v", err)
	}
}


func TestBuildDefaultOutput(t *testing.T) {
	tmpDir := t.TempDir()
	vbxFile := filepath.Join(tmpDir, "testapp.vbx")
	content := []byte("Dim x = 10\nPrint x * 2")
	if err := os.WriteFile(vbxFile, content, 0644); err != nil {
		t.Fatalf("Failed to write temp vbx file: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}
	defer os.Chdir(origDir)

	builtPath, err := Build("testapp.vbx", "", false)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if _, err := os.Stat(builtPath); os.IsNotExist(err) {
		t.Fatalf("Expected built binary at %s, but file does not exist", builtPath)
	}

	cFilePath := strings.TrimSuffix(builtPath, filepath.Ext(builtPath)) + ".c"
	if _, err := os.Stat(cFilePath); !os.IsNotExist(err) {
		t.Errorf("Expected intermediate .c file %s to be removed, but it exists", cFilePath)
	}

	cmd := exec.Command("./" + builtPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Executing built binary failed: %v, output: %s", err, string(output))
	}
	if !strings.Contains(string(output), "20") {
		t.Errorf("Expected binary output to contain '20', got: %s", string(output))
	}
}

func TestBuildCustomOutputAndKeepC(t *testing.T) {
	tmpDir := t.TempDir()
	vbxFile := filepath.Join(tmpDir, "source.vbx")
	content := []byte("Print \"Hello Build\"")
	if err := os.WriteFile(vbxFile, content, 0644); err != nil {
		t.Fatalf("Failed to write temp vbx file: %v", err)
	}

	customOut := filepath.Join(tmpDir, "custom_bin")
	builtPath, err := Build(vbxFile, customOut, true)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if _, err := os.Stat(builtPath); os.IsNotExist(err) {
		t.Fatalf("Expected built binary at %s, but file does not exist", builtPath)
	}

	ext := filepath.Ext(builtPath)
	expectedCFile := strings.TrimSuffix(builtPath, ext) + ".c"
	if _, err := os.Stat(expectedCFile); os.IsNotExist(err) {
		t.Errorf("Expected intermediate .c file %s to exist with keepC=true, but it does not", expectedCFile)
	}

	cmd := exec.Command(builtPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Executing built binary failed: %v, output: %s", err, string(output))
	}
	if !strings.Contains(string(output), "Hello Build") {
		t.Errorf("Expected binary output to contain 'Hello Build', got: %s", string(output))
	}
}

func TestBuildPopupExample(t *testing.T) {
	popupPath := filepath.Join("..", "..", "examples", "popup.vbx")
	if _, err := os.Stat(popupPath); os.IsNotExist(err) {
		t.Skip("examples/popup.vbx not found")
	}

	tmpDir := t.TempDir()
	outBin := filepath.Join(tmpDir, "popup_standalone")

	builtPath, err := Build(popupPath, outBin, false)
	if err != nil {
		t.Fatalf("Build popup.vbx failed: %v", err)
	}

	if _, err := os.Stat(builtPath); os.IsNotExist(err) {
		t.Fatalf("Expected built popup binary at %s, but file does not exist", builtPath)
	}

	cmd := exec.Command(builtPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Executing popup standalone binary failed: %v, output: %s", err, string(output))
	}

	expectedSnippet := "Hello from Visual Basic X Popup!"
	if !strings.Contains(string(output), expectedSnippet) {
		t.Errorf("Expected popup output to contain %q, got: %s", expectedSnippet, string(output))
	}
}

func TestTranspileInputBox(t *testing.T) {
	tmpDir := t.TempDir()
	vbxFile := filepath.Join(tmpDir, "test_inputbox.vbx")
	content := []byte(`
Dim input1 = InputBox("Enter prompt 1")
Dim input2 = InputBox("Enter prompt 2", "Title 2")
Print input1 + " " + input2
`)
	if err := os.WriteFile(vbxFile, content, 0644); err != nil {
		t.Fatalf("Failed to write temp vbx file: %v", err)
	}

	cCode, err := Transpile(vbxFile)
	if err != nil {
		t.Fatalf("Transpile failed: %v", err)
	}

	expectedSnippets := []string{
		`vbx_inputbox("Enter prompt 1", NULL)`,
		`vbx_inputbox("Enter prompt 2", "Title 2")`,
		"static char* vbx_inputbox",
		"#include <stdio.h>",
		"#include <stdlib.h>",
		"#include <string.h>",
	}

	for _, snippet := range expectedSnippets {
		if !strings.Contains(cCode, snippet) {
			t.Errorf("Expected snippet %q in C code, but not found.\nGenerated C code:\n%s", snippet, cCode)
		}
	}
}

func TestTranspileFileOperations(t *testing.T) {
	tmpDir := t.TempDir()
	vbxFile := filepath.Join(tmpDir, "test_file_io.vbx")
	content := []byte(`
File.Write "sample.txt", "Sample file content"
Dim filename = "sample2.txt"
Dim body = "Second file content"
File.Write(filename, body)
Dim read1 = File.Read("sample.txt")
Dim read2 = File.Read(filename)
Print read1
Print read2
`)
	if err := os.WriteFile(vbxFile, content, 0644); err != nil {
		t.Fatalf("Failed to write temp vbx file: %v", err)
	}

	cCode, err := Transpile(vbxFile)
	if err != nil {
		t.Fatalf("Transpile failed: %v", err)
	}

	expectedSnippets := []string{
		`vbx_file_write("sample.txt", "Sample file content");`,
		`vbx_file_write(filename, body);`,
		`vbx_file_read("sample.txt")`,
		`vbx_file_read(filename)`,
		"static void vbx_file_write",
		"static char* vbx_file_read",
	}

	for _, snippet := range expectedSnippets {
		if !strings.Contains(cCode, snippet) {
			t.Errorf("Expected snippet %q in C code, but not found.\nGenerated C code:\n%s", snippet, cCode)
		}
	}
}

func TestBuildAndRunFileOperations(t *testing.T) {
	tmpDir := t.TempDir()
	vbxFile := filepath.Join(tmpDir, "run_file_io.vbx")
	outFile := filepath.Join(tmpDir, "output_test.txt")
	// Escape backslashes for path on Windows if needed
	cleanOutFile := strings.ReplaceAll(outFile, "\\", "/")

	content := []byte(strings.Join([]string{
		`Dim filePath = "` + cleanOutFile + `"`,
		`File.Write filePath, "VBX File I/O Success!"`,
		`Dim readBack = File.Read(filePath)`,
		`Print "Read content: " + readBack`,
	}, "\n"))

	if err := os.WriteFile(vbxFile, content, 0644); err != nil {
		t.Fatalf("Failed to write temp vbx file: %v", err)
	}

	err := BuildAndRun(vbxFile)
	if err != nil {
		t.Fatalf("BuildAndRun failed: %v", err)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("Expected file %s to exist, but got error: %v", outFile, err)
	}

	if string(data) != "VBX File I/O Success!" {
		t.Errorf("Expected file content 'VBX File I/O Success!', got: %s", string(data))
	}
}

func TestBuildInteractiveAppExample(t *testing.T) {
	interactiveAppPath := filepath.Join("..", "..", "examples", "interactive_app.vbx")
	if _, err := os.Stat(interactiveAppPath); os.IsNotExist(err) {
		t.Skip("examples/interactive_app.vbx not found")
	}

	tmpDir := t.TempDir()
	outBin := filepath.Join(tmpDir, "interactive_app_standalone")

	builtPath, err := Build(interactiveAppPath, outBin, false)
	if err != nil {
		t.Fatalf("Build interactive_app.vbx failed: %v", err)
	}

	if _, err := os.Stat(builtPath); os.IsNotExist(err) {
		t.Fatalf("Expected built interactive_app binary at %s, but file does not exist", builtPath)
	}
}

func TestTranspileWhileLoop(t *testing.T) {
	tmpDir := t.TempDir()
	vbxFile := filepath.Join(tmpDir, "test_while.vbx")
	content := []byte(`
Dim count = 3
While count > 0
    Print count
    count = count - 1
Wend
`)
	if err := os.WriteFile(vbxFile, content, 0644); err != nil {
		t.Fatalf("Failed to write temp vbx file: %v", err)
	}

	cCode, err := Transpile(vbxFile)
	if err != nil {
		t.Fatalf("Transpile failed: %v", err)
	}

	expectedSnippets := []string{
		"while ((count > 0LL)) {",
		"count = (count - 1LL);",
		"}",
	}

	for _, snippet := range expectedSnippets {
		if !strings.Contains(cCode, snippet) {
			t.Errorf("Expected snippet %q in C code, but not found.\nGenerated C code:\n%s", snippet, cCode)
		}
	}
}

func TestTranspileElseIf(t *testing.T) {
	tmpDir := t.TempDir()
	vbxFile := filepath.Join(tmpDir, "test_elseif.vbx")
	content := []byte(`
Dim score = 85
If score >= 90 Then
    Print "A"
ElseIf score >= 80 Then
    Print "B"
ElseIf score >= 70 Then
    Print "C"
Else
    Print "F"
End If
`)
	if err := os.WriteFile(vbxFile, content, 0644); err != nil {
		t.Fatalf("Failed to write temp vbx file: %v", err)
	}

	cCode, err := Transpile(vbxFile)
	if err != nil {
		t.Fatalf("Transpile failed: %v", err)
	}

	expectedSnippets := []string{
		"if ((score >= 90LL)) {",
		"} else if ((score >= 80LL)) {",
		"} else if ((score >= 70LL)) {",
		"} else {",
		"}",
	}

	for _, snippet := range expectedSnippets {
		if !strings.Contains(cCode, snippet) {
			t.Errorf("Expected snippet %q in C code, but not found.\nGenerated C code:\n%s", snippet, cCode)
		}
	}
}

func TestTranspileStringConcatAndFunctions(t *testing.T) {
	tmpDir := t.TempDir()
	vbxFile := filepath.Join(tmpDir, "test_str_fn.vbx")
	content := []byte(`
Dim num = 10
Dim msg = "Count: " & num & " items"
Dim u = UCase("hello")
Dim l = LCase("WORLD")
Dim leftStr = Left("Visual", 2)
Dim rightStr = Right("Basic", 3)
Dim midStr = Mid("Transpiler", 2, 4)
Dim length = Len("Test")
Print msg
Print u & l & leftStr & rightStr & midStr
Print length
`)
	if err := os.WriteFile(vbxFile, content, 0644); err != nil {
		t.Fatalf("Failed to write temp vbx file: %v", err)
	}

	cCode, err := Transpile(vbxFile)
	if err != nil {
		t.Fatalf("Transpile failed: %v", err)
	}

	expectedSnippets := []string{
		`const char* msg = vbx_concat(vbx_concat("Count: ", vbx_int_to_str(num)), " items");`,
		`const char* u = vbx_ucase("hello");`,
		`const char* l = vbx_lcase("WORLD");`,
		`const char* leftStr = vbx_left("Visual", 2LL);`,
		`const char* rightStr = vbx_right("Basic", 3LL);`,
		`const char* midStr = vbx_mid("Transpiler", 2LL, 4LL);`,
		`long long length = ((long long)strlen("Test"));`,
		"static char* vbx_ucase",
		"static char* vbx_lcase",
		"static char* vbx_left",
		"static char* vbx_right",
		"static char* vbx_mid",
	}

	for _, snippet := range expectedSnippets {
		if !strings.Contains(cCode, snippet) {
			t.Errorf("Expected snippet %q in C code, but not found.\nGenerated C code:\n%s", snippet, cCode)
		}
	}
}

func TestTranspileNewFeatureErrors(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "Wend without While",
			content: "Wend",
		},
		{
			name:    "Unclosed While block",
			content: "While 1 == 1\nPrint 1",
		},
		{
			name:    "ElseIf without If",
			content: "ElseIf 1 == 1 Then\nPrint 1\nEnd If",
		},
		{
			name:    "ElseIf after Else",
			content: "If 1 == 1 Then\nPrint 1\nElse\nPrint 2\nElseIf 2 == 2 Then\nPrint 3\nEnd If",
		},
		{
			name:    "Len with invalid args",
			content: "Dim x = Len()",
		},
		{
			name:    "Mid with invalid arg count",
			content: `Dim s = Mid("abc", 1)`,
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

func TestBuildAndRunStringAndLoopsExample(t *testing.T) {
	examplePath := filepath.Join("..", "..", "examples", "string_and_loops.vbx")
	if _, err := os.Stat(examplePath); os.IsNotExist(err) {
		t.Skip("examples/string_and_loops.vbx not found")
	}

	err := BuildAndRun(examplePath)
	if err != nil {
		t.Fatalf("BuildAndRun examples/string_and_loops.vbx failed: %v", err)
	}
}
