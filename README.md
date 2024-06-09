# xmlmonster
In short, it's a victim to ddos attacks by POSTing many huge xmls at the same time

## Running
Just type in the commands below
```bash
cd <to_your_xmlmonster_dir>
docker compose up -d 
go build main.go
export CERT_FILE=<path_to_your_cert_file>
export KEY_FILE=<path_to_your_key_file>
./main
```

## Supported endpoints
Server has two endpoints, namely /upload and /read. 
The first one takes in xml file and saves it to S3 storage, while the second handles reading objects from it 