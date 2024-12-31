create database sqld;
use sqld;
CREATE TABLE data (`id` int NOT NULL AUTO_INCREMENT, `key` text NOT NULL, `value` text NOT NULL, `active` BOOLEAN NOT NULL DEFAULT TRUE, PRIMARY KEY (`id`));
