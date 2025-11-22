DROP TABLE balances;

CREATE TABLE balances (
    id SERIAL PRIMARY KEY,
    amount INTEGER NOT NULL
);

INSERT INTO balances (amount) VALUES (10), (50), (200), (12), (220)