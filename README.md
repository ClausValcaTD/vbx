# ⚡ VBX (Visual Basic X)

<p align="center">
  <img src="https://img.shields.io/badge/Language-Visual%20Basic%20X-blue?style=for-the-badge&logo=visualbasic" alt="Language" />
  <img src="https://img.shields.io/badge/License-GPLv3%20with%20Exception-success?style=for-the-badge" alt="License" />
  <img src="https://img.shields.io/badge/Version-0.1.0--alpha-orange?style=for-the-badge" alt="Version" />
  <img src="https://img.shields.io/badge/Platform-Windows%20%7C%20macOS%20%7C%20Linux-lightgrey?style=for-the-badge" alt="Platform" />
</p>

<p align="center">
  <b>"The simplicity you remember. The power you need."</b><br/>
  <i>Reviving the beloved simplicity of Visual Basic 6 as a modern, cross-platform, zero-dependency compiled language.</i>
</p>

---

## 🌟 Why VBX?

Remember when writing a program was simple, visual, and genuinely fun? You didn't need to configure complex bundlers, download 500MB of `node_modules`, or battle dependency hell just to display a window or calculate some numbers. 

**VBX** brings back the magic of **Rapid Application Development (RAD)** for the modern era:
- 🚀 **Blazing Native Speed:** Transpiles `.vbx` code into clean standard **C99** and compiles with `-O2` optimization.
- 📦 **Zero-Dependency Standalone Binaries:** Generates single `.exe` / executable binaries. No virtual environments, no runtimes required.
- 🪟 **Iconic Native GUI:** Built-in `MsgBox` and `InputBox` that display native OS dialogs across Windows, macOS, and Linux without external GUI bloat.
- 🧘 **Human-Friendly Syntax:** The classic keywords you know (`Dim`, `Sub`, `Function`, `For ... To ... Next`, `While ... Wend`, `If ... ElseIf ... Else ... End If`) with modern automatic type inference.
- 🔤 **Rich Built-in Strings:** Native `&` concatenation, plus classic functions (`Len`, `UCase`, `LCase`, `Left`, `Right`, `Mid`).
- 💾 **Built-in Standard Essentials:** Native File I/O (`File.Write`, `File.Read`) right out of the box.

---

## 💻 Quick Showcase

### 1. Interactive Apps with Native Popups & File I/O
Prompt the user, greet them, and save data to disk in less than 15 lines:

```vbx
' Interactive Registration App
Dim userName = InputBox("Please enter your name:", "Registration")

' Display native alert
MsgBox "Hello, " & userName & "! Saving your data...", "Welcome"

' Save to local file
Dim logFile = "user_data.txt"
File.Write logFile, "Registered User: " & userName

' Read it back and verify
Dim savedContent = File.Read(logFile)
MsgBox "Verified from file: " & savedContent, "Success"
```

### 2. Powerful Control Flow & String Functions
```vbx
Dim msg = "Visual Basic X"
Print "Length: " & Len(msg)
Print "Upper: " & UCase(msg)
Print "Sub: " & Mid(msg, 1, 6)

' While ... Wend Loop
Dim count = 3
While count > 0
    Print "Countdown: " & count
    count = count - 1
Wend

' ElseIf Condition
Dim score = 85
If score >= 90 Then
    Print "Grade: A"
ElseIf score >= 80 Then
    Print "Grade: B"
Else
    Print "Grade: C"
End If
```

### 3. Procedures & Functions
```vbx
Sub Greet(name)
    MsgBox "Welcome to VBX, " & name
End Sub

Function CalculateTotal(price, taxRate)
    Return price + (price * taxRate)
End Function

Greet "Developer"
Dim total = CalculateTotal(100.0, 0.14)
Print "Total: " & total
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
# Builds popup.exe (Windows) or popup (Linux/macOS)
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
 Generated Standard C99 Code (Lean Helpers)
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
- [x] Control Flow (`If-ElseIf-Else-End If`, `For-To-Next`, `While-Wend`)
- [x] Subroutines (`Sub`) & Functions (`Function`)
- [x] Cross-Platform Native Dialogs (`MsgBox`, `InputBox`)
- [x] Built-in String Library (`&`, `Len`, `UCase`, `LCase`, `Left`, `Right`, `Mid`)
- [x] Standard File I/O (`File.Write`, `File.Read`)
- [x] Standalone Binary Compiler (`vbx build` with `-O2`)
- [ ] 1D Arrays (`Dim arr(size)`)
- [ ] VS Code Extension (Syntax Highlighting & Run integration)
- [ ] Lightweight Dedicated Visual IDE (**VBX Studio**)
- [ ] Embedded SQLite3 Database Support

---

## 📜 License

This project is licensed under the **GNU General Public License v3.0 with a Runtime Library Exception** - see the [LICENSE](LICENSE) file for details.

> **Note on Compiler Exception:** Programs, applications, and binaries created using the VBX compiler are **NOT** subject to the GPL. You are free to distribute, license, or sell your compiled VBX software under any terms you choose (including closed-source and proprietary).

---

<p align="center">
  <i>"Software shouldn't be a chore. It should be a joy to build."</i><br/>
  <b>M5 Dev — 2026</b>
</p>
