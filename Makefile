run-wrapper:
	AUTH_KEY=supersecret go run ./cmd/minecraft-server-wrapper

run-center:
	AUTH_KEY=supersecret go run ./cmd/minecraft-server-center

run-docker:
	docker build -t gogo-mc-bedrock-server .
	docker run -it --rm -p 8080:8080 -p 19132:19132/udp -e EULA_ACCEPT=true -e AUTH_KEY=supersecret gogo-mc-bedrock-server