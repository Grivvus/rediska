## yet-another clone of redis

### build & run
```
sh your_program.sh --port 6379 # run master on 6379 tcp port
sh your_program.sh --port 6380 --replicaof "0.0.0.0 6379" # run replica on 6380 tcp port
```

### work still in progress
