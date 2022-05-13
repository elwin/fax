build:
	GOOS=linux GOARCH=arm go build

push:
	scp printer pi:~/printer