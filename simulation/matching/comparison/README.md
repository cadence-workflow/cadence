
This tool runs a set of matching simulation tests, extracts stats from their output and generates a csv to compare them easily.

Note: The parsing logic might break in the future if the `simulation/matching/run.sh` starts spitting different shaped lines. Alternative is to load all the event logs into a sqlite table and then run queries on top instead of parsing outputs of jq in this tool.


Run all the scenarios and compare:
```
go run simulation/matching/comparison/*.go
```

Run subset of scenarios and compare:
```
go run simulation/matching/comparison/*.go \
    --scenarios "fluctuating"
```

If you have already run some scenarios before and made changes in the csv output then run in Compare mode
```
go run simulation/matching/comparison/*.go \
    --ts 2024-11-27-21-29-55 \
    --mode Compare
```
