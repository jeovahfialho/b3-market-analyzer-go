-- Extensões necessárias
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
CREATE EXTENSION IF NOT EXISTS btree_gin;

-- Remover tabela existente se houver
DROP TABLE IF EXISTS trades CASCADE;
DROP MATERIALIZED VIEW IF EXISTS daily_aggregations CASCADE;

-- Tabela principal particionada
CREATE TABLE trades (
    id BIGSERIAL,
    hora_fechamento TIME NOT NULL,
    data_negocio DATE NOT NULL,
    codigo_instrumento VARCHAR(20) NOT NULL,
    preco_negocio DECIMAL(10, 2) NOT NULL,
    quantidade_negociada BIGINT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, data_negocio)
) PARTITION BY RANGE (data_negocio);

-- Criar partições manualmente para 2025
CREATE TABLE trades_2025_05 PARTITION OF trades 
FOR VALUES FROM ('2025-05-01') TO ('2025-06-01');

CREATE TABLE trades_2025_06 PARTITION OF trades 
FOR VALUES FROM ('2025-06-01') TO ('2025-07-01');

CREATE TABLE trades_2025_07 PARTITION OF trades 
FOR VALUES FROM ('2025-07-01') TO ('2025-08-01');

-- Índices nas partições
CREATE INDEX trades_2025_05_ticker_date_idx ON trades_2025_05 USING btree (codigo_instrumento, data_negocio);
CREATE INDEX trades_2025_06_ticker_date_idx ON trades_2025_06 USING btree (codigo_instrumento, data_negocio);
CREATE INDEX trades_2025_07_ticker_date_idx ON trades_2025_07 USING btree (codigo_instrumento, data_negocio);

-- Índices GIN
CREATE INDEX trades_2025_05_gin_idx ON trades_2025_05 USING gin (codigo_instrumento, data_negocio, preco_negocio);
CREATE INDEX trades_2025_06_gin_idx ON trades_2025_06 USING gin (codigo_instrumento, data_negocio, preco_negocio);
CREATE INDEX trades_2025_07_gin_idx ON trades_2025_07 USING gin (codigo_instrumento, data_negocio, preco_negocio);

-- Materialized View para agregações
CREATE MATERIALIZED VIEW daily_aggregations AS
SELECT
    codigo_instrumento,
    data_negocio,
    MAX(preco_negocio) as max_price,
    SUM(quantidade_negociada) as total_volume,
    COUNT(*) as trade_count,
    MIN(preco_negocio) as min_price,
    AVG(preco_negocio) as avg_price,
    STDDEV(preco_negocio) as price_stddev
FROM trades
GROUP BY codigo_instrumento, data_negocio;

-- Índice único para refresh concorrente
CREATE UNIQUE INDEX daily_agg_unique_idx
ON daily_aggregations(codigo_instrumento, data_negocio);

-- Índices adicionais para performance
CREATE INDEX daily_agg_ticker_idx ON daily_aggregations(codigo_instrumento);
CREATE INDEX daily_agg_date_idx ON daily_aggregations(data_negocio);
CREATE INDEX daily_agg_volume_idx ON daily_aggregations(total_volume DESC);