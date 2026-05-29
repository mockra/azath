Output demo/azath.gif

Set Shell bash
Set FontSize 18
Set Width 1200
Set Height 720
Set Theme "Catppuccin Mocha"
Set TypingSpeed 50ms
Set PlaybackSpeed 1.0

Hide
Type "bash demo/teardown.sh && source demo/setup.sh && clear" Enter
Sleep 1s
Show

Type "azath --help" Sleep 500ms Enter
Sleep 4s

Type "clear" Enter
Sleep 300ms

Type "azath list" Sleep 500ms Enter
Sleep 3s

Type "azath show api" Sleep 500ms Enter
Sleep 3s

Type "azath config | head -20" Sleep 500ms Enter
Sleep 3s

Type "azath sweep --dry-run" Sleep 500ms Enter
Sleep 2s

Type "azath doctor" Sleep 500ms Enter
Sleep 3s

Hide
Type "bash demo/teardown.sh" Enter
