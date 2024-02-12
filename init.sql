CREATE TABLE content
(
    id           INTEGER PRIMARY KEY,
    path         VARCHAR(1024) NOT NULL UNIQUE,
    content_type VARCHAR(255)  NOT NULL,
    content      BLOB          NOT NULL,
    created_at   DATETIME      NOT NULL,
    updated_at   DATETIME      NULL,
    accessed_at  DATETIME      NULL
)
