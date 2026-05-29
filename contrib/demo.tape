# VHS tape for the azath dashboard demo.
# Render with: vhs contrib/demo.tape
Output contrib/demo.gif

Set Shell "bash"
Set FontSize 16
Set Width 1100
Set Height 650
Set Theme "Catppuccin Mocha"
Set Padding 18
Set TypingSpeed 60ms
Set PlaybackSpeed 1.0

Hide
Type "clear" Enter
Show

Type "# azath: a tmux + Copilot CLI orchestrator"
Sleep 1s
Enter

Type "azath list --plain"
Sleep 600ms
Enter
Sleep 2s

Type "azath dash"
Sleep 600ms
Enter
Sleep 2500ms

Type "azath"
Sleep 1500ms

Down@250ms 3
Sleep 1500ms

Up@250ms 2
Sleep 1500ms

Ctrl+R
Sleep 1500ms

Escape
Sleep 1500ms
