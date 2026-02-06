module github.com/sa6mwa/pslog/elevatorpitch

go 1.25

replace pkt.systems/pslog => ../

replace github.com/sa6mwa/pslog/benchmark => ../benchmark

require (
	github.com/phuslu/log v1.0.121
	github.com/rs/zerolog v1.34.0
	github.com/sa6mwa/pslog/benchmark v0.0.0-00010101000000-000000000000
	go.uber.org/zap v1.27.1
	pkt.systems/pslog v0.0.0-00010101000000-000000000000
)

require (
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
)
