CREATE SCHEMA simf;

CREATE TABLE credito_inmediato_enviado (
    id bigserial not null,
    name varchar not null,
    last_name varchar not null,
    st_respst_negoci character varying(1) not null
);