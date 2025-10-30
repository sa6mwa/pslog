Output elevatorpitch.gif
Set Width 780
Set Height 620
Set FontSize 14
Set Margin 4
Set Padding 10
Set MarginFill "#674EFF"
Set BorderRadius 12

Set Shell "bash"
Env PS1 "\[\e[0;34m\]❯\[\e[0m\] "

Set TypingSpeed 0.05

Type "./pslog-elevator-pitch"
Enter
Wait+Line /^❯\s*$/
Sleep 10s
