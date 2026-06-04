# T-Sync CLI

T-Sync is a high-performance, secure backup tool designed for selective, incremental synchronization. This CLI client supports Curve25519-based encryption, multiple remote storage backends (including OCI, S3, and Local Storage), and interactive TUI file browsers/selectors.

---

## Installation

You can install the pre-compiled, optimized `tsync` executable on any target platform using the following one-liner commands.

### 🐧 Linux

#### AMD64 (x86_64)
```bash
sudo curl -fsSL -o /usr/local/bin/tsync https://github.com/abyii/t-sync-cli/releases/latest/download/tsync-linux-amd64 && sudo chmod +x /usr/local/bin/tsync
```

#### ARM64 (Aarch64)
```bash
sudo curl -fsSL -o /usr/local/bin/tsync https://github.com/abyii/t-sync-cli/releases/latest/download/tsync-linux-arm64 && sudo chmod +x /usr/local/bin/tsync
```

---

### 🍏 macOS

#### Apple Silicon (M1/M2/M3 / ARM64)
```bash
sudo curl -fsSL -o /usr/local/bin/tsync https://github.com/abyii/t-sync-cli/releases/latest/download/tsync-darwin-arm64 && sudo chmod +x /usr/local/bin/tsync
```

#### Intel (AMD64)
```bash
sudo curl -fsSL -o /usr/local/bin/tsync https://github.com/abyii/t-sync-cli/releases/latest/download/tsync-darwin-amd64 && sudo chmod +x /usr/local/bin/tsync
```

---

### 🪟 Windows

The Windows one-liners download the executable directly into your user's AppExecutionAlias directory (`%LOCALAPPDATA%\Microsoft\WindowsApps`), which is in your Windows `PATH` by default. **No Administrator privileges or manual PATH environment configurations are required.**

#### AMD64 (x86_64)
Run the following command in **PowerShell**:
```powershell
Invoke-WebRequest -Uri "https://github.com/abyii/t-sync-cli/releases/latest/download/tsync-windows-amd64.exe" -OutFile "$env:LOCALAPPDATA\Microsoft\WindowsApps\tsync.exe"
```

#### ARM64
Run the following command in **PowerShell**:
```powershell
Invoke-WebRequest -Uri "https://github.com/abyii/t-sync-cli/releases/latest/download/tsync-windows-arm64.exe" -OutFile "$env:LOCALAPPDATA\Microsoft\WindowsApps\tsync.exe"
```

---

## Verifying the Installation

Open a new terminal window on your machine and run the following command to check that the installation was successful:

```bash
tsync --help
```

---

## Local Development & Compilation

To build and compile the project from source locally:

1. Clone the repository:
   ```bash
   git clone https://github.com/abyii/t-sync-cli.git
   cd t-sync-cli
   ```
2. Build the binary using the local installer script (for Windows):
   ```powershell
   powershell -File .\install.ps1
   ```
   Or run the Go compiler directly:
   ```bash
   go build -o tsync main.go
   ```
3. Run the E2E test suite:
   ```bash
   go test -v ./...
   ```
