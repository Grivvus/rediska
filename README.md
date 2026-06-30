### rediska - a toy Redis clone

### how to use

```bash
# from one terminal run the database
go run ./cmd/rediska/main.go
```

```bash
# from other terminal you can run cli redis client
redis-cli SET 321 abacaba  # output: OK
redis-cli GET 321  # output: abacaba
```

### what's ready

- [X] PING, GET, SET commands
- [ ] RDB persistance
- [ ] replication
