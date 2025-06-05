# B3 Market Analyzer - Guia de Instalação com Docker 🚀

Sistema de alta performance para análise de dados do mercado B3, implementado em Go com foco em eficiência e escalabilidade.

## 📋 Pré-requisitos

Antes de começar, certifique-se de ter instalado:

- **Docker** 24.x ou superior
- **Docker Compose** 2.x ou superior
- **Git** (para clonar o repositório)

### Verificando as instalações

```bash
# Verificar Docker
docker --version
# Esperado: Docker version 24.x.x ou superior

# Verificar Docker Compose
docker-compose --version
# Esperado: Docker Compose version 2.x.x ou superior
```

## 🚀 Quick Start

### 1. Clone o repositório

```bash
git clone https://github.com/jeovahfialho/b3-market-analyzer-go.git
cd b3-market-analyzer-go
```

### 2. Prepare o ambiente

```bash
# Criar diretório para dados
mkdir -p data

# Dar permissões corretas aos scripts
chmod 755 scripts/*.sql 2>/dev/null || true
```

### 3. Subir os containers

```bash
# Entrar no diretório docker
cd docker

# Subir todos os serviços em modo detached
docker-compose up -d

# Verificar status dos containers
docker-compose ps
```

### 4. Aplicar schema do banco

```bash
# Aguardar PostgreSQL ficar ready (cerca de 30 segundos)
sleep 30

# Aplicar schema (volte para a raiz do projeto)
cd ..
docker cp scripts/schema.sql b3_postgres:/tmp/schema.sql
docker-compose -f docker/docker-compose.yml exec postgres psql -U b3user -d b3_market -f /tmp/schema.sql
```

## 📊 Verificando a instalação

### 1. Verificar status dos containers

```bash
# Ver status dos containers
cd docker
docker-compose ps
```

Você deve ver algo como:
```
NAME                 STATUS              PORTS
b3_api              running             0.0.0.0:8000->8000/tcp
b3_postgres         running (healthy)   0.0.0.0:5432->5432/tcp
b3_redis            running (healthy)   0.0.0.0:6379->6379/tcp
b3_prometheus       running             0.0.0.0:9090->9090/tcp
b3_grafana          running             0.0.0.0:3000->3000/tcp
```

### 2. Verificar se o schema foi aplicado

```bash
# Verificar se as tabelas foram criadas
docker-compose exec postgres psql -U b3user -d b3_market -c "\dt"

# Deve mostrar as tabelas: trades, partições e daily_aggregations
```

### 3. Testar a aplicação

```bash
# Testar health check
curl http://localhost:8000/health

# Resposta esperada:
{
  "status": "ok"
}
```

## 🌐 Acessando as interfaces

Após subir os containers, você pode acessar:

- **API**: http://localhost:8000
- **Swagger/Docs**: http://localhost:8000/swagger/index.html
- **Prometheus**: http://localhost:9090
- **Grafana**: http://localhost:3000 
  - Login: `admin`
  - Senha: `admin`

## 📥 Carregando dados da B3

### Usando a CLI dentro do container

```bash
# Entrar no container da API
docker-compose exec api sh

# Dentro do container:

# 1. Baixar dados dos últimos 7 dias úteis da B3
./b3-analyzer-cli download --days 7

# 2. Listar arquivos baixados
./b3-analyzer-cli list

# 3. Carregar os arquivos TXT no banco
./b3-analyzer-cli load data/*.txt

# 4. Consultar dados de um ticker
./b3-analyzer-cli query PETR4

# 5. Consultar com filtro de data
./b3-analyzer-cli query PETR4 --start-date 2025-05-20

# 6. Sair do container
exit
```

### Executar comandos diretamente (sem entrar no container)

```bash
# Download de dados
docker-compose exec api ./b3-analyzer-cli download --days 7

# Carregar dados
docker-compose exec api ./b3-analyzer-cli load data/*.txt

# Verificar dados carregados
docker-compose exec api ./b3-analyzer-cli query PETR4
```

## 🔍 Testando com dados reais

Após carregar os dados:

### API REST

```bash
# Consultar agregações de um ticker
curl "http://localhost:8000/api/v1/ticker/PETR4/aggregation"

# Com filtro de data
curl "http://localhost:8000/api/v1/ticker/PETR4/aggregation?start_date=2025-05-20"

# Resposta esperada:
{
  "ticker": "PETR4",
  "max_range_value": 42.50,
  "max_daily_volume": 15000000
}
```

### Swagger UI

1. Acesse: http://localhost:8000/swagger/index.html
2. Teste os endpoints interativamente
3. Veja a documentação completa da API

## 💻 Desenvolvimento Local (Fora do Container)

Para desenvolvimento e debug, você pode rodar a aplicação localmente usando apenas PostgreSQL e Redis no Docker:

### 1. Preparar ambiente local

```bash
# Manter apenas banco e cache no Docker
cd docker
docker-compose down
docker-compose up -d postgres redis

# Aguardar serviços subirem
sleep 10

# Verificar se PostgreSQL está funcionando
docker-compose exec postgres psql -U b3user -d b3_market -c "SELECT 1;"
```

### 2. Aplicar schema do banco

```bash
# Voltar para raiz do projeto
cd ..

# Aplicar schema
docker cp scripts/schema.sql b3_postgres:/tmp/schema.sql
docker-compose -f docker/docker-compose.yml exec postgres psql -U b3user -d b3_market -f /tmp/schema.sql

# Verificar se tabelas foram criadas
docker-compose -f docker/docker-compose.yml exec postgres psql -U b3user -d b3_market -c "\dt"
```

### 3. Configurar variáveis de ambiente

Crie um arquivo `local-env.sh` na raiz do projeto:

```bash
#!/bin/bash

# Configurar ambiente local
export DATABASE_URL="postgres://b3user:b3pass@localhost:5432/b3_market?sslmode=disable"
export REDIS_URL="redis://localhost:6379"
export LOG_LEVEL="info"
export API_HOST="0.0.0.0"
export API_PORT="8000"
export BATCH_SIZE="10000"
export WORKERS="4"
export CACHE_TTL="1h"

echo "✅ Variáveis de ambiente configuradas!"
echo "🐘 PostgreSQL: localhost:5432"
echo "🔴 Redis: localhost:6379"
echo "🌐 API: localhost:8000"
```

Torne executável e configure o ambiente:
```bash
chmod +x local-env.sh
source local-env.sh
```

### 4. Compilar aplicações

```bash
# Compilar CLI
go build -o b3-analyzer-cli cmd/cli/main.go

# Compilar API
go build -o b3-analyzer-api cmd/api/main.go

# Verificar se compilou
ls -la b3-analyzer-*
```

### 5. Testar CLI

```bash
# Testar conectividade
./b3-analyzer-cli health

# Download dados da B3
./b3-analyzer-cli download --days 3 --output ./data

# Listar arquivos baixados
./b3-analyzer-cli list --dir ./data

# Carregar dados no banco
./b3-analyzer-cli load data/*.txt

# Consultar dados
./b3-analyzer-cli query PETR4

# Consultar com filtro de data
./b3-analyzer-cli query PETR4 --start-date 2025-05-20
```

### 6. Rodar API (terminal separado)

```bash
# Em outro terminal, configurar ambiente
source local-env.sh

# Rodar API
./b3-analyzer-api

# Deve mostrar:
# Server starting on 0.0.0.0:8000
```

### 7. Testar endpoints API

```bash
# Health check
curl http://localhost:8000/health

# Teste agregação
curl "http://localhost:8000/api/v1/ticker/PETR4/aggregation"

# Swagger JSON
curl http://localhost:8000/swagger/doc.json

# Abrir Swagger UI no navegador
open http://localhost:8000/swagger/index.html
```

### 8. Script completo para desenvolvimento

Crie um arquivo `dev-helper.sh` para automatizar o setup local:

```bash
#!/bin/bash

case "$1" in
  setup)
    echo "🚀 Configurando ambiente de desenvolvimento..."
    cd docker
    docker-compose down
    docker-compose up -d postgres redis
    echo "⏳ Aguardando serviços..."
    sleep 15
    cd ..
    echo "📊 Aplicando schema..."
    docker cp scripts/schema.sql b3_postgres:/tmp/schema.sql
    docker-compose -f docker/docker-compose.yml exec postgres psql -U b3user -d b3_market -f /tmp/schema.sql
    echo "🔧 Compilando aplicações..."
    source local-env.sh
    go build -o b3-analyzer-cli cmd/cli/main.go
    go build -o b3-analyzer-api cmd/api/main.go
    echo "✅ Ambiente pronto!"
    echo "💡 Execute: source local-env.sh && ./dev-helper.sh test"
    ;;
  
  compile)
    echo "🔧 Compilando..."
    go build -o b3-analyzer-cli cmd/cli/main.go
    go build -o b3-analyzer-api cmd/api/main.go
    echo "✅ Compilação concluída!"
    ;;
  
  test-cli)
    echo "🧪 Testando CLI..."
    ./b3-analyzer-cli health
    echo "📥 Baixando 1 dia de dados para teste..."
    ./b3-analyzer-cli download --days 1 --output ./data
    ./b3-analyzer-cli list --dir ./data
    echo "💾 Carregando dados..."
    ./b3-analyzer-cli load data/*.txt
    echo "🔍 Testando query..."
    ./b3-analyzer-cli query PETR4
    ;;
  
  run-api)
    echo "🌐 Iniciando API..."
    echo "💡 Acesse: http://localhost:8000/swagger/index.html"
    ./b3-analyzer-api
    ;;
  
  test-api)
    echo "🧪 Testando API..."
    echo "Health check:"
    curl -s http://localhost:8000/health | jq 2>/dev/null || curl -s http://localhost:8000/health
    echo -e "\n\nTeste agregação:"
    curl -s "http://localhost:8000/api/v1/ticker/PETR4/aggregation" | jq 2>/dev/null || curl -s "http://localhost:8000/api/v1/ticker/PETR4/aggregation"
    echo -e "\n\nSwagger JSON:"
    curl -s http://localhost:8000/swagger/doc.json | head -c 100
    echo "..."
    ;;
  
  logs)
    echo "📊 Logs do PostgreSQL:"
    cd docker && docker-compose logs postgres | tail -20
    ;;
  
  db)
    echo "🐘 Acessando PostgreSQL..."
    cd docker && docker-compose exec postgres psql -U b3user -d b3_market
    ;;
  
  redis)
    echo "🔴 Acessando Redis..."
    cd docker && docker-compose exec redis redis-cli
    ;;
  
  clean)
    echo "🧹 Limpando ambiente..."
    rm -f b3-analyzer-cli b3-analyzer-api
    rm -rf data/*
    cd docker && docker-compose down -v
    ;;
  
  reset-schema)
    echo "🔄 Reaplicando schema..."
    docker cp scripts/schema.sql b3_postgres:/tmp/schema.sql
    cd docker && docker-compose exec postgres psql -U b3user -d b3_market -f /tmp/schema.sql
    ;;
  
  *)
    echo "Uso: $0 {setup|compile|test-cli|run-api|test-api|logs|db|redis|clean|reset-schema}"
    echo ""
    echo "Comandos de desenvolvimento:"
    echo "  setup      - Configurar ambiente completo de desenvolvimento"
    echo "  compile    - Compilar CLI e API"
    echo "  test-cli   - Testar CLI com download e carga de dados"
    echo "  run-api    - Executar API localmente"
    echo "  test-api   - Testar endpoints da API"
    echo "  logs       - Ver logs do PostgreSQL"
    echo "  db         - Acessar shell do PostgreSQL"
    echo "  redis      - Acessar shell do Redis"
    echo "  clean      - Limpar tudo"
    echo "  reset-schema - Reaplicar schema do banco"
    echo ""
    echo "Fluxo típico:"
    echo "  1. ./dev-helper.sh setup"
    echo "  2. source local-env.sh"
    echo "  3. ./dev-helper.sh test-cli"
    echo "  4. ./dev-helper.sh run-api  (em outro terminal)"
    echo "  5. ./dev-helper.sh test-api (em terceiro terminal)"
    exit 1
    ;;
esac
```

Tornar executável:
```bash
chmod +x dev-helper.sh
```

### 9. Fluxo completo de desenvolvimento

```bash
# Configuração inicial (uma vez)
./dev-helper.sh setup

# Configurar variáveis de ambiente
source local-env.sh

# Testar CLI
./dev-helper.sh test-cli

# Em outro terminal: rodar API
source local-env.sh
./dev-helper.sh run-api

# Em terceiro terminal: testar API
./dev-helper.sh test-api

# Acessar Swagger UI
open http://localhost:8000/swagger/index.html
```

### Vantagens do desenvolvimento local

- ✅ **Debug direto**: Breakpoints e logs imediatos
- ✅ **Compilação rápida**: Sem rebuild de containers
- ✅ **Hot reload**: Recompile e execute rapidamente
- ✅ **Logs claros**: Output direto no terminal
- ✅ **Performance**: Execução nativa sem overhead de container
- ✅ **Facilita TDD**: Testes unitários e integração

### Troubleshooting desenvolvimento local

#### Erro de conexão com banco:
```bash
# Verificar se PostgreSQL está rodando
cd docker && docker-compose ps postgres

# Testar conexão
docker-compose exec postgres psql -U b3user -d b3_market -c "SELECT 1;"

# Verificar logs
docker-compose logs postgres
```

#### Erro de compilação:
```bash
# Atualizar dependências
go mod tidy

# Compilar com verbose para ver erros
go build -v -o b3-analyzer-cli cmd/cli/main.go
```

#### Swagger não funciona:
```bash
# Instalar swag se não tiver
go install github.com/swaggo/swag/cmd/swag@latest

# Gerar documentação
swag init -g cmd/api/main.go -o docs

# Recompilar API
go build -o b3-analyzer-api cmd/api/main.go
```

#### Erro "table does not exist":
```bash
# Reaplicar schema
./dev-helper.sh reset-schema

# Verificar se tabelas foram criadas
./dev-helper.sh db
\dt
\q
```

## 🛠️ Comandos úteis

### Gerenciamento dos containers

```bash
# Parar todos os serviços
docker-compose down

# Parar e remover volumes (limpa todos os dados)
docker-compose down -v

# Reconstruir imagens
docker-compose build --no-cache

# Reconstruir e subir
docker-compose up -d --build

# Reiniciar um serviço específico
docker-compose restart api

# Ver uso de recursos
docker stats
```

### Executar comandos nos containers

```bash
# Acessar shell do container da API
docker-compose exec api sh

# Executar query no PostgreSQL
docker-compose exec postgres psql -U b3user -d b3_market

# Acessar Redis CLI
docker-compose exec redis redis-cli

# Ver logs específicos
docker-compose logs -f api
docker-compose logs postgres
```

### Análise de dados

```bash
# Ver estatísticas das partições
docker-compose exec postgres psql -U b3user -d b3_market -c "
SELECT 
    schemaname,
    tablename,
    n_tup_ins as inserts,
    n_tup_upd as updates,
    n_tup_del as deletes,
    n_live_tup as live_rows
FROM pg_stat_user_tables 
WHERE tablename LIKE 'trades_%';"

# Atualizar materialized view manualmente
docker-compose exec api ./b3-analyzer-cli refresh

# Ver top tickers por volume
docker-compose exec postgres psql -U b3user -d b3_market -c "
SELECT 
    codigo_instrumento,
    SUM(total_volume) as volume_total,
    COUNT(*) as dias_negociacao
FROM daily_aggregations 
GROUP BY codigo_instrumento 
ORDER BY volume_total DESC 
LIMIT 10;"
```

## 🚨 Troubleshooting

### Problema: Erro ao carregar dados - "character varying(12)"

```bash
# Ajustar tamanho do campo codigo_instrumento se necessário
docker-compose exec postgres psql -U b3user -d b3_market -c "
ALTER TABLE trades_2025_05 ALTER COLUMN codigo_instrumento TYPE VARCHAR(30);
ALTER TABLE trades_2025_06 ALTER COLUMN codigo_instrumento TYPE VARCHAR(30);
ALTER TABLE trades_2025_07 ALTER COLUMN codigo_instrumento TYPE VARCHAR(30);
"
```

### Problema: Tabela não existe

```bash
# Verificar se schema foi aplicado
docker-compose exec postgres psql -U b3user -d b3_market -c "\dt"

# Se não existe, aplicar novamente
docker cp scripts/schema.sql b3_postgres:/tmp/schema.sql
docker-compose exec postgres psql -U b3user -d b3_market -f /tmp/schema.sql
```

### Problema: Container não inicia

```bash
# Ver logs detalhados
docker-compose logs api | grep ERROR

# Verificar saúde dos containers
docker inspect b3_api | grep -i health

# Reconstruir do zero
docker-compose down -v
docker-compose build --no-cache
docker-compose up -d
```

### Problema: Swagger retorna erro 500

```bash
# Verificar se docs foram gerados no build
docker-compose exec api ls -la docs/

# Verificar endpoint direto
curl http://localhost:8000/swagger/doc.json

# Se necessário, rebuild
docker-compose build --no-cache api
docker-compose up -d api
```

### Problema: Portas em uso

```bash
# Verificar se as portas estão livres
lsof -i :8000  # API
lsof -i :5432  # PostgreSQL
lsof -i :6379  # Redis
lsof -i :3000  # Grafana
lsof -i :9090  # Prometheus

# Matar processo usando a porta (exemplo)
kill -9 $(lsof -t -i:8000)
```

## 📝 Script helper

Para facilitar o uso diário, crie um arquivo `docker-helper.sh`:

```bash
#!/bin/bash

case "$1" in
  start)
    echo "🚀 Iniciando B3 Market Analyzer..."
    cd docker && docker-compose up -d
    echo "⏳ Aguardando serviços..."
    sleep 30
    echo "📊 Aplicando schema..."
    cd .. && docker cp scripts/schema.sql b3_postgres:/tmp/schema.sql
    docker-compose -f docker/docker-compose.yml exec postgres psql -U b3user -d b3_market -f /tmp/schema.sql
    echo "✅ Serviços iniciados!"
    echo "📊 API: http://localhost:8000"
    echo "📈 Grafana: http://localhost:3000"
    echo "📋 Swagger: http://localhost:8000/swagger/index.html"
    ;;
  
  stop)
    echo "🛑 Parando serviços..."
    cd docker && docker-compose down
    ;;
  
  logs)
    cd docker && docker-compose logs -f api
    ;;
  
  load-data)
    echo "📥 Carregando dados da B3..."
    cd docker
    docker-compose exec api ./b3-analyzer-cli download --days 7
    docker-compose exec api ./b3-analyzer-cli load data/*.txt
    docker-compose exec api ./b3-analyzer-cli query PETR4
    ;;
  
  test)
    echo "🧪 Testando API..."
    curl -s http://localhost:8000/health
    echo ""
    curl -s "http://localhost:8000/api/v1/ticker/PETR4/aggregation"
    ;;
  
  status)
    cd docker && docker-compose ps
    ;;
  
  clean)
    echo "🧹 Limpando tudo..."
    cd docker && docker-compose down -v
    ;;
  
  rebuild)
    echo "🔨 Reconstruindo..."
    cd docker
    docker-compose down
    docker-compose build --no-cache
    docker-compose up -d
    sleep 30
    cd .. && docker cp scripts/schema.sql b3_postgres:/tmp/schema.sql
    docker-compose -f docker/docker-compose.yml exec postgres psql -U b3user -d b3_market -f /tmp/schema.sql
    ;;
  
  schema)
    echo "📊 Aplicando schema..."
    docker cp scripts/schema.sql b3_postgres:/tmp/schema.sql
    cd docker && docker-compose exec postgres psql -U b3user -d b3_market -f /tmp/schema.sql
    ;;
  
  *)
    echo "Uso: $0 {start|stop|logs|load-data|test|status|clean|rebuild|schema}"
    echo ""
    echo "Comandos:"
    echo "  start      - Iniciar todos os serviços"
    echo "  stop       - Parar todos os serviços" 
    echo "  logs       - Ver logs da API"
    echo "  load-data  - Baixar e carregar dados da B3"
    echo "  test       - Testar endpoints da API"
    echo "  status     - Ver status dos containers"
    echo "  clean      - Remover tudo (incluindo dados)"
    echo "  rebuild    - Rebuild completo"
    echo "  schema     - Aplicar schema do banco"
    exit 1
    ;;
esac
```

Torne o script executável:
```bash
chmod +x docker-helper.sh
```

Uso:
```bash
./docker-helper.sh start      # Iniciar serviços completos
./docker-helper.sh load-data  # Carregar dados da B3
./docker-helper.sh test       # Testar API
./docker-helper.sh logs       # Ver logs
./docker-helper.sh stop       # Parar serviços
```

## ✅ Verificação final

Se tudo estiver funcionando corretamente, você verá:

1. ✅ Todos os containers rodando (`docker-compose ps`)
2. ✅ Health check retornando "ok" 
3. ✅ Schema aplicado (tabelas `trades` e partições criadas)
4. ✅ Dados carregados com sucesso
5. ✅ API respondendo consultas de agregação
6. ✅ Swagger acessível em http://localhost:8000/swagger/index.html
7. ✅ Grafana acessível em http://localhost:3000
8. ✅ Logs sem erros (`docker-compose logs`)

## 🎯 Exemplo de Fluxo Completo

```bash
# 1. Iniciar tudo
./docker-helper.sh start

# 2. Carregar dados reais da B3
./docker-helper.sh load-data

# 3. Testar API
curl "http://localhost:8000/api/v1/ticker/PETR4/aggregation"

# 4. Acessar Swagger
# Abrir: http://localhost:8000/swagger/index.html

# 5. Ver métricas no Grafana  
# Abrir: http://localhost:3000 (admin/admin)
```

## 🎉 Pronto!

Seu B3 Market Analyzer está rodando com Docker! 🚀🐳

### Próximos passos

1. **Carregar dados reais**: Use a CLI para baixar dados da B3
2. **Explorar a API**: Acesse o Swagger e teste os endpoints
3. **Configurar dashboards**: Acesse o Grafana e crie visualizações
4. **Otimizar performance**: Monitore via Prometheus

## 📚 Recursos do Sistema

### Arquivos Suportados
- **Fonte**: B3 (Bolsa de Valores brasileira)
- **Formato**: TXT com separador `;` (ponto-vírgula)
- **URL**: https://arquivos.b3.com.br/rapinegocios/tickercsv
- **Frequência**: Diários (dias úteis)

### Performance
- **Particionamento**: Tabelas particionadas por data
- **Índices otimizados**: GIN e BTREE para queries rápidas
- **Cache Redis**: Agregações em cache para respostas instantâneas
- **Bulk loading**: COPY FROM PostgreSQL para máxima velocidade

### Agregações Disponíveis
- **max_range_value**: Maior preço individual negociado
- **max_daily_volume**: Maior volume diário total
- Filtros por ticker e período
