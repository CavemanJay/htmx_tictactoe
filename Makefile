build: format
	@templ generate
	@go build -o ./tmp/main.exe cmd/main.go

format:
	@templ fmt .