# ⚡ VBX (Visual Basic X)

<p align="center">
  <img src="https://img.shields.io/badge/Language-Visual%20Basic%20X-blue?style=for-the-badge&logo=visualbasic" alt="Language" />
  <img src="https://img.shields.io/badge/License-MIT-green?style=for-the-badge" alt="License" />
  <img src="https://img.shields.io/badge/Version-0.1.0--alpha-orange?style=for-the-badge" alt="Version" />
  <img src="https://img.shields.io/badge/Platform-Windows%20%7C%20macOS%20%7C%20Linux-lightgrey?style=for-the-badge" alt="Platform" />
</p>

<p align="center">
  <b>Reviving the beloved simplicity of Visual Basic 6 as a modern, cross-platform, zero-dependency compiled language.</b>
</p>

---

## 🌟 Why VBX?

Remember when writing a program was simple, visual, and genuinely fun? You didn't need to configure complex bundlers, download 500MB of `node_modules`, or battle dependency hell just to display a window or calculate some numbers. 

**VBX** brings back the magic of **Rapid Application Development (RAD)** for the modern era:
- 🚀 **Blazing Native Speed:** Transpiles `.vbx` code into optimized standard **C99** and compiles with `-O2`.
- 📦 **Zero-Dependency Standalone Binaries:** Single `.exe` / executable output. No virtual environments, no runtimes required.
- 🪟 **Iconic Native GUI:** Built-in `MsgBox` and `InputBox` that display native OS dialogs across Windows, macOS, and Linux without third-party frameworks.
- 🧘 **Human-Friendly Syntax:** The beloved keywords you know (`Dim`, `Sub`, `Function`, `For ... To ... Next`, `If ... Then ... End If`) with modern automatic type inference.
- 💾 **Built-in Standard Essentials:** Native File I/O (`File.Write`, `File.Read`) out of the box.

---

## 💻 Quick Showcase

### 1. The Classic Interactive Experience
Prompt the user, greet them, and save data to disk in less than 15 lines:

```vbx
' Interactive Registration App
Dim userName = InputBox("Please enter your name:", "Registration")

' Display native alert
MsgBox "Hello, " + userName + "! Saving your data...", "Welcome"

' Save to local file
Dim logFile = "user_data.txt"
File.Write logFile, "Registered User: " + userName

' Read it back and verify
Dim savedContent = File.Read(logFile)
MsgBox "Verified from file: " + savedContent, "Success"
```

### 2. Procedures & Functions
```vbx
Sub Greet(name)
    MsgBox "Welcome to VBX, " + name
End Sub

Function CalculateTotal(price, taxRate)
    Return price + (price * taxRate)
End Function

Greet "Developer"

Dim finalPrice = CalculateTotal(100.0, 0.14)
Print "Final Price: " + finalPrice
```

### 3. Loops & Conditions
```vbx
For i = 1 To 5
    If i % 2 == 0 Then
        Print i + " is Even"
    Else
        Print i + " is Odd"
    End If
Next i
```

---

## 🛠️ Getting Started

### Prerequisites
- [Go](https://golang.org) (1.20+) to build the compiler.
- A standard C compiler installed (`gcc` or `clang`) on your system PATH.

### Installation
Clone the repository and build the `vbx` CLI:

```bash
git clone https://github.com/M5Devs/vbx.git
cd vbx
go build -o vbx ./cmd/vbx
```

*(On Windows, this produces `vbx.exe`)*

---

## 🚀 CLI Usage

### Run a script instantly
Transpile, compile, and execute in one command:
```bash
vbx run examples/interactive_app.vbx
```

### Build a standalone binary
Generate a production-ready, optimized standalone executable:
```bash
# Builds app.exe (Windows) or app (Linux/macOS)
vbx build examples/popup.vbx

# Specify custom output name
vbx build examples/popup.vbx -o my_cool_app

# Keep intermediate generated C code for inspection
vbx build examples/popup.vbx --keep-c
```

### Version check
```bash
vbx version
```

---

## 🏛️ Architecture Overview

```
.vbx Source Code 
       │
       ▼
 [ VBX Parser & Type Inference (Go) ]
       │
       ▼
 Generated Standard C99 Code
       │
       ▼
 [ C Compiler (GCC / Clang with -O2) ]
       │
       ▼
 Ultra-fast Standalone Native Binary (.exe)
```

---

## 🗺️ Roadmap to 1.0

- [x] Recursive Descent Parser & Expression AST
- [x] Dynamic Type Inference (`long long`, `double`, `const char*`)
- [x] Control Flow (`If-Else-End If`, `For-To-Next`)
- [x] Subroutines (`Sub`) & Functions (`Function`)
- [x] Cross-Platform Native Dialogs (`MsgBox`, `InputBox`)
- [x] Standard File I/O (`File.Write`, `File.Read`)
- [x] Standalone Binary Compiler (`vbx build` with `-O2`)
- [ ] VS Code Extension (Syntax Highlighting & Run integration)
- [ ] Lightweight Dedicated Visual IDE (**VBX Studio**)
- [ ] Embedded SQLite3 Database Support

---

## 📜 License

This project is licensed under the **MIT License** - see the [LICENSE](LICENSE) file for details.

---

<p align="center">
  <i>"Software shouldn't be a chore. It should be a joy to build."</i><br/>
  <b>Visual Basic X Team — 2026</b>
</p>
