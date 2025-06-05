-- Extensões necessárias
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
CREATE EXTENSION IF NOT EXISTS btree_gin;

-- Tabela principal particionada
CREATE TABLE trades (
    id BIGSERIAL,
    hora_fechamento TIME NOT NULL,
    data_negocio DATE NOT NULL,
    codigo_instrumento VARCHAR(12) NOT NULL,
    preco_negocio DECIMAL(10, 2) NOT NULL,
    quantidade_negociada BIGINT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, data_negocio)
) PARTITION BY RANGE (data_negocio);

-- Função para criar partições automaticamente
CREATE OR REPLACE FUNCTION create_monthly_partitions()
RETURNS void AS $
DECLARE
    start_date date;
    end_date date;
    partition_name text;
BEGIN
    FOR i IN 0..11 LOOP
        start_date := date_trunc('month', CURRENT_DATE - interval '1 month' * i);
        end_date := start_date + interval '1 month';
        partition_name := 'trades_' || to_char(start_date, 'YYYY_MM');
        
        IF NOT EXISTS (
            SELECT 1 FROM pg_tables WHERE tablename = partition_name
        ) THEN
            EXECUTE format(
                'CREATE TABLE %I PARTITION OF trades FOR VALUES FROM (%L) TO (%L)',
                partition_name, start_date, end_date
            );
            
            -- Índices na partição
            EXECUTE format(
                'CREATE INDEX %I ON %I USING btree (codigo_instrumento, data_negocio)',
                partition_name || '_ticker_date_idx',
                partition_name
            );
            
            -- Índice GIN para queries complexas
            EXECUTE format(
                'CREATE INDEX %I ON %I USING gin (codigo_instrumento, data_negocio, preco_negocio)',
                partition_name || '_gin_idx',
                partition_name
            );
        END IF;
    END LOOP;
END;
$ LANGUAGE plpgsql;

-- Executa criação de partições
SELECT create_monthly_partitions();

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

-- Configurações de performance
ALTER SYSTEM SET shared_buffers = '512MB';
ALTER SYSTEM SET effective_cache_size = '2GB';
ALTER SYSTEM SET maintenance_work_mem = '256MB';
ALTER SYSTEM SET work_mem = '32MB';
ALTER SYSTEM SET max_worker_processes = 8;
ALTER SYSTEM SET max_parallel_workers_per_gather = 4;
ALTER SYSTEM SET max_parallel_workers = 8;
ALTER SYSTEM SET random_page_cost = 1.1;
ALTER SYSTEM SET effective_io_concurrency = 200;