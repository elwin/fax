build:
	GOOS=linux GOARCH=arm go build -o dist/fax

push:
	scp dist/fax pi:~/fax