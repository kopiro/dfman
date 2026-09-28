package dfman

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf16"
)

func ps(ctx context.Context, script string) (string, error) {
	u := utf16.Encode([]rune("$ProgressPreference='SilentlyContinue'\n" + script))
	b := make([]byte, len(u)*2)
	for i, v := range u {
		binary.LittleEndian.PutUint16(b[i*2:], v)
	}
	return runCommand(ctx, "", nil, "powershell.exe", "-NoProfile", "-NonInteractive", "-EncodedCommand", base64.StdEncoding.EncodeToString(b))
}
func psQuote(s string) string  { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
func winQuote(s string) string { return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"` }
func desktopNotify(ctx context.Context, title, body, key string) error {
	p, e := helperPath("dfman-notify.exe")
	if e != nil {
		return e
	}
	_, e = runCommand(ctx, "", nil, p, "send", title, body, key)
	return e
}
func installAgent(ctx context.Context, exe, config, state string, c Config, out io.Writer) error {
	helper, e := helperPath("dfman-notify.exe")
	if e != nil {
		return e
	}
	if c.Notification {
		if _, e = runCommand(ctx, "", nil, helper, "register", exe); e != nil {
			return e
		}
	}
	d, _ := c.Interval()
	args := "run " + winQuote(exe) + " --config " + winQuote(config) + " --state-dir " + winQuote(state) + " agent run"
	script := fmt.Sprintf(`$ErrorActionPreference='Stop'
$a=New-ScheduledTaskAction -Execute %s -Argument %s
$t=New-ScheduledTaskTrigger -Once -At (Get-Date).AddMinutes(%d) -RepetitionInterval (New-TimeSpan -Minutes %d)
$p=New-ScheduledTaskPrincipal -UserId ([Security.Principal.WindowsIdentity]::GetCurrent().Name) -LogonType Interactive -RunLevel Limited
$s=New-ScheduledTaskSettingsSet -MultipleInstances IgnoreNew -StartWhenAvailable -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -ExecutionTimeLimit (New-TimeSpan -Minutes 10)
Register-ScheduledTask -TaskName 'dfman-agent' -Action $a -Trigger $t -Principal $p -Settings $s -Force | Out-Null`, psQuote(helper), psQuote(args), int(d/time.Minute), int(d/time.Minute))
	if _, e = ps(ctx, script); e != nil {
		return e
	}
	fmt.Fprintf(out, "Installed agent: every %s (%s).\n", d, c.Agent.SyncMode)
	return nil
}
func uninstallAgent(ctx context.Context, out io.Writer) error {
	_, e := ps(ctx, `$ErrorActionPreference='Stop'; if (Get-ScheduledTask -TaskName 'dfman-agent' -ErrorAction SilentlyContinue) { Stop-ScheduledTask -TaskName 'dfman-agent'; Unregister-ScheduledTask -TaskName 'dfman-agent' -Confirm:$false }`)
	if e != nil {
		return e
	}
	if helper, err := helperPath("dfman-notify.exe"); err == nil {
		if _, err = runCommand(ctx, "", nil, helper, "unregister"); err != nil {
			return err
		}
	}
	fmt.Fprintln(out, "Agent uninstalled.")
	return nil
}
func platformAgentStatus(ctx context.Context, out io.Writer) error {
	s, e := ps(ctx, `$ErrorActionPreference='Stop'; Get-ScheduledTask -TaskName 'dfman-agent' | Format-List TaskName,State,Principal; Get-ScheduledTaskInfo -TaskName 'dfman-agent' | Format-List LastRunTime,LastTaskResult,NextRunTime`)
	fmt.Fprintln(out, s)
	return e
}
