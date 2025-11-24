module github.com/sa6mwa/pslog/examples

go 1.25.2

replace pkt.systems/pslog => ../

require pkt.systems/pslog v0.0.0-00010101000000-000000000000

require (
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/term v0.37.0 // indirect
)
