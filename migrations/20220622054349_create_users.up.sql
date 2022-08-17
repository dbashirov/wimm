CREATE TABLE users (
    id bigserial not null primary key,
    username varchar(200) not null,
    email varchar(200) not null unique,
    encrypted_password varchar not null
);