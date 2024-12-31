create database sqld;
use sqld;
CREATE TABLE data (id integer primary key generated always as identity, key text  NOT NULL, value text NOT NULL, active bool NOT NULL DEFAULT(TRUE));
