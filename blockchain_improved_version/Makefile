# Makefile for building miner and deploying/starting server scripts

# Define the targets and their dependencies

all: miner server

miner: ./miner/miner.go
	go build -o miner ./miner/miner.go

server: ./scripts/deploy_server.sh ./scripts/start_server.sh
	cd ./scripts && bash deploy_server.sh 5 && bash start_server.sh 5

# Define a clean target to remove built binaries

clean:
	rm -f miner

# PHONY targets to avoid conflicts with file names

.PHONY: all miner server clean