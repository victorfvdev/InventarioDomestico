# Estoque

Inventário doméstico com núcleo de produtos/movimentações, **carga inicial por foto (RN01)**, **entrada por NFC-e MT / Cuiabá (RN02)** e **baixa por texto + atalhos (RN04/RN06)**.

## O que este repositório entrega

- API Go (monólito modular) + PostgreSQL
- Produtos com status **estimado** vs **confirmado**
- Movimentações imutáveis: entrada manual, ajuste, carga inicial, `purchase_entry`, `consumption`
- Fluxo A: foto → visão (mock sem OpenAI) → confirmação → inventário
- Fluxo B: chave/QR NFC-e MT → itens → match de produtos → confirmação → estoque
- Fluxo C: texto PT-BR ou atalho → revisão → confirmação → `consumption` (desfazer via ajuste ≠ consumo)
- Idempotência: a mesma chave de 44 dígitos nunca republica saldo
- Fallbacks: fixture offline Cuiabá, colar chave, itens manuais, stub de foto sem OpenAI, parser local PT-BR

## Pré-requisitos

- Go 1.22+
- Node.js 20+
- PostgreSQL 16 (local ou via Docker)

## Subir o banco

```bash
docker compose up -d postgres
# ou PostgreSQL local com usuário/senha/db: estoque / estoque / estoque
```

## Configuração

```bash
cp .env.example .env
# OPENAI_API_KEY é opcional (visão, cupom e consumo usam mock/parser local sem chave)
# NFCE_FORCE_FIXTURE=true força o HTML offline MT sem tentar SEFAZ
```

## API (porta 3847)

```bash
cd api
export DATABASE_URL=postgres://estoque:estoque@127.0.0.1:5432/estoque?sslmode=disable
export HTTP_ADDR=:3847
export CORS_ORIGINS=http://127.0.0.1:3848,http://localhost:3848
go run ./cmd/server
```

Healthcheck: `GET http://127.0.0.1:3847/health`

## Web (porta 3848)

```bash
cd web
export NEXT_PUBLIC_API_URL=http://127.0.0.1:3847
npm install
npm run dev -- -p 3848 -H 127.0.0.1
```

Abra [http://127.0.0.1:3848](http://127.0.0.1:3848).

## Baixa por texto e atalhos (EST-5)

1. Cadastre produtos (foto, NFC-e ou entrada manual)
2. Abra [http://127.0.0.1:3848/consumo](http://127.0.0.1:3848/consumo)
3. Escreva algo como `Comi duas bananas.` → revise matches → **Confirmar baixa**
4. Use atalhos (histórico ou sementes) com confirmação explícita
5. **Desfazer** gera `inventory_adjust` (não conta como consumo)

Via API:

```bash
curl -s -X POST http://127.0.0.1:3847/api/consumption/interpret \
  -H 'content-type: application/json' \
  -d '{"text":"Comi duas bananas."}' | jq
curl -s http://127.0.0.1:3847/api/consumption/shortcuts | jq
```

Parser: `api/internal/consumption` (local PT-BR, sem OpenAI). OpenAI fica opcional para o futuro.

## NFC-e Mato Grosso (EST-4)

### Caminho de demo com fixture (recomendado)

1. Abra [http://127.0.0.1:3848/nfce](http://127.0.0.1:3848/nfce)
2. Clique em **Usar fixture Cuiabá**
3. Revise matches e confirme — gera `purchase_entry`
4. Repetir a mesma chave não duplica estoque

### Consulta real SEFAZ-MT — o que funciona vs o que é bloqueado

| Caminho | Status |
| --- | --- |
| Fixture offline / parse HTML | Funciona (demos e testes) |
| Colar chave 44 dígitos ou URL do QR | Funciona (extrai chave; tenta SEFAZ ou cai na fixture) |
| Entrada manual de itens | Funciona |
| Foto do cupom | Stub local **sem** OpenAI |
| Consulta automática SEFAZ-MT ao vivo | Tentada; com frequência bloqueada por CAPTCHA/HTML |

## Fluxo rápido

1. **Carga por foto** — inventário inicial confirmado
2. **NFC-e MT** — compra via fixture ou chave
3. **Consumo** — texto ou atalho com confirmação humana
4. **Entrada manual** / **Ajustar** — correções pontuais
5. **Movimentações** — auditoria

## Testes

```bash
cd api && go test ./internal/consumption/ ./internal/service/ -run 'Parse|Rank|Consumption' ./internal/nfce/
```

## Fora deste slice

Visão OpenAI real, receitas, voz, multi-usuário e lista de compras.
