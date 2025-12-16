module github.com/sa6mwa/pslog/examples

go 1.24.0

replace pkt.systems/pslog => ../

require pkt.systems/pslog v0.0.0-00010101000000-000000000000

require (
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/term v0.38.0 // indirect
)
