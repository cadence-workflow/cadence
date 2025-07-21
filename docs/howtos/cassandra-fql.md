# View Cassandra full query logs

1. Run cadence components via docker compose
```
docker compose docker/docker-compose.yml up
```

2. First enable fql via nodetool ([ref](https://cassandra.apache.org/doc/stable/cassandra/operating/fqllogging.html#enabling-fql))
```
docker exec $(docker inspect --format="{{.Id}}" docker-cassandra-1) nodetool enablefullquerylog --path /tmp/cassandra_fql
```

3. Check `/tmp/cassandra_fql` folder exists
```
docker exec $(docker inspect --format="{{.Id}}" docker-cassandra-1) ls /tmp/cassandra_fql
```

4. Run some workflows to generate queries (optional)

5. Inspect the full query log dump via fqltool (it's under tools/bin in default apache cassandra installation)
```
docker cp $(docker inspect --format="{{.Id}}" docker-cassandra-1):/tmp/cassandra_fql/ /tmp/cassandra_fql/ && fqltool dump /tmp/cassandra_fql | less
```
