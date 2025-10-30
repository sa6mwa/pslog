Output demo.gif
Set Width 700
Set Height 160
Set FontSize 10
Set Margin 4
Set Padding 5
Set MarginFill "#674EFF"
Set BorderRadius 12
Set WaitTimeout 30s

Set Shell "bash"
Env PS1 "\[\e[0;34m\]❯\[\e[0m\] "

Set TypingSpeed 0.05

Type "./demo"
Enter
Wait+Line /^❯\s*$/
Sleep 5s
