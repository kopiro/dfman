package dfman

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

func installUpdate(ctx context.Context, payload, dest string, out io.Writer) (bool, error) {
	// A separate process waits for this executable to exit before replacing it.
	script := `$ErrorActionPreference='Stop'
Wait-Process -Id ` + strconv.Itoa(os.Getpid()) + ` -ErrorAction SilentlyContinue
$payload=` + psQuote(payload) + `
$dest=` + psQuote(dest) + `
$backup=Join-Path (Split-Path $payload) 'previous'
New-Item -ItemType Directory -Path $backup | Out-Null
$moved=@();$installed=@()
try {
 foreach($name in @('dfman.exe','dfman-notify.exe')) {
  $target=Join-Path $dest $name
  if(Test-Path $target){Move-Item -LiteralPath $target -Destination (Join-Path $backup $name);$moved+=$name}
  Move-Item -LiteralPath (Join-Path $payload $name) -Destination $target
  $installed+=$name
 }
 'Update installed' | Set-Content (Join-Path $dest 'dfman-update.log')
 Remove-Item -LiteralPath (Split-Path $payload) -Recurse -Force
} catch {
 foreach($name in $installed){Remove-Item -LiteralPath (Join-Path $dest $name) -Force -ErrorAction SilentlyContinue}
 foreach($name in $moved){Move-Item -LiteralPath (Join-Path $backup $name) -Destination (Join-Path $dest $name) -Force}
 $_ | Out-String | Set-Content (Join-Path $dest 'dfman-update.log')
 exit 1
}`
	path := filepath.Join(filepath.Dir(payload), "update.ps1")
	if e := os.WriteFile(path, []byte(script), 0600); e != nil {
		return false, e
	}
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", path)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000200 | 0x08000000, HideWindow: true}
	if e := cmd.Start(); e != nil {
		return false, e
	}
	cmd.Process.Release()
	fmt.Fprintln(out, "Update staged; replacement starts when dfman exits. Result: dfman-update.log beside the executable.")
	return true, nil
}
