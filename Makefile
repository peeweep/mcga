run:
	go build -ldflags "-s -w " -trimpath -o mcga
	./mcga -d=talon-service-prod.ecosec.on.epicgames.com -s=dns.opendns.com -p=5353 -f=my.hosts
	./mcga -d=cloudflare.com -s=dns.opendns.com -p=5353 -f=my.hosts
	./mcga -d=visa.cn -s=dns.opendns.com -p=5353 -f=my.hosts -m=true
