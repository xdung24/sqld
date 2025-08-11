create database sqld;

\c sqld

-- Create public schema table
CREATE TABLE data (id integer primary key generated always as identity, key text  NOT NULL, value text NOT NULL, active bool NOT NULL DEFAULT(TRUE));

-- Create a test schema
CREATE SCHEMA test_schema;

-- Create table in test schema
CREATE TABLE test_schema.products (
    id integer primary key generated always as identity, 
    name text NOT NULL, 
    price decimal(10,2) NOT NULL, 
    category text NOT NULL,
    created_at timestamp DEFAULT CURRENT_TIMESTAMP
);

-- Insert some test data
INSERT INTO test_schema.products (name, price, category) VALUES 
    ('Laptop', 999.99, 'Electronics'),
    ('Coffee Mug', 12.50, 'Kitchen'),
    ('Notebook', 5.99, 'Office');
