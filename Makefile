bin:
	GOOS=linux go build -o main
	zip smspub-deployment.zip main

clean:
	rm -f smspub-deployment.zip
