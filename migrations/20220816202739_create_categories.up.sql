CREATE TABLE categories (
    id bigserial not null primary key,
    title varchar(150) not null,
    id_user bigserial not null,
    type int not null
);