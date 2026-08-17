CREATE TABLE shards (
    shard_id INT NOT NULL,
    --
    range_id BIGINT NOT NULL,
    data BLOB NOT NULL,
    data_encoding VARCHAR(16) NOT NULL,
    PRIMARY KEY (shard_id)
);