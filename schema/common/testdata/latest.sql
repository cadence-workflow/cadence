CREATE TABLE shards (
    shard_id INT NOT NULL,
    --
    range_id BIGINT NOT NULL,
    data BLOB NOT NULL,
    data_encoding VARCHAR(16) NOT NULL,
    PRIMARY KEY (shard_id)
);

CREATE TABLE domain_metadata (
     `id` bigint(20) NOT NULL AUTO_INCREMENT,
     notification_version BIGINT NOT NULL,
     PRIMARY KEY (`id`)
);

-- This doesn't match v0.2 and that's okay
INSERT INTO domain_metadata (notification_version) VALUES (1);

