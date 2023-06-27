run:
	go build -ldflags "-s -w " -trimpath -o mcga
	./mcga talon-service-prod.ecosec.on.epicgames.com -u dns.opendns.com -p 5353
