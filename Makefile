run:
	go run ./cmd/minecraft-server-wrapper

run-docker:
	docker build -t gogo-mc-bedrock-server .
	docker run -it --rm -p 8080:8080 -p 19132:19132/udp -e EULA_ACCEPT=true gogo-mc-bedrock-server