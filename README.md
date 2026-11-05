# Estoque

Inventário doméstico com núcleo de produtos/movimentações, **carga inicial por foto (RN01)** e **entrada por NFC-e MT / Cuiabá (RN02)**.

## O que este repositório entrega

- API Go (monólito modular) + PostgreSQL
- Produtos com status **estimado** vs **confirmado**
- Movimentações imutáveis: entrada manual, ajuste, carga inicial, `purchase_entry`
- Fluxo A: foto → visão (mock sem OpenAI) → confirmação → inventário
- Fluxo B: chave/QR NFC-e MT → itens → match de produtos → confirmação → estoque
- Idempotência: a mesma chave de 44 dígitos nunca republica saldo
- Fallbacks: fixture offline Cuiabá, colar chave, itens manuais, stub de foto sem OpenAI

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
# OPENAI_API_KEY é opcional (visão e cupom usam mock/stub sem chave)
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

## NFC-e Mato Grosso (EST-4)

### Caminho de demo com fixture (recomendado)

1. Abra [http://127.0.0.1:3848/nfce](http://127.0.0.1:3848/nfce)
2. Clique em **Usar fixture Cuiabá** (Atacadão · chave `51241103887073000156650010001234561123456780`)
3. Revise matches (exato / sugestão / ambiguidade / novo)
4. Confirme — gera movimentações `purchase_entry`
5. Repita a mesma chave: a API devolve o documento já lançado e **não** duplica estoque

Via API:

```bash
curl -s http://127.0.0.1:3847/api/nfce/fixture | jq
curl -s -X POST http://127.0.0.1:3847/api/nfce/lookup \
  -H 'content-type: application/json' \
  -d '{"useFixture":true}' | jq '.document.accessKey,.usedFixture,.notes'
```

Fixture HTML: `api/internal/nfce/fixture/cuiaba-atacadao.html`

### Consulta real SEFAZ-MT — o que funciona vs o que é bloqueado

| Caminho | Status |
| --- | --- |
| Fixture offline / parse HTML | Funciona (demos e testes) |
| Colar chave 44 dígitos ou URL do QR | Funciona (extrai chave; tenta SEFAZ ou cai na fixture) |
| Entrada manual de itens | Funciona |
| Foto do cupom | Stub local **sem** OpenAI (itens típicos para revisão) |
| Consulta automática SEFAZ-MT ao vivo | Tentada (`GET` na consulta pública); com frequência **bloqueada** por CAPTCHA, sessão/cookies, HTML sem tabela de itens ou indisponibilidade |

Limites documentados no adapter (`api/internal/nfce/adapter.go`) e em `GET /api/nfce/fixture`.

## Fluxo rápido (Slice C + EST-4)

1. **Carga por foto** — inventário inicial confirmado
2. **NFC-e MT** — compra via fixture ou chave
3. **Entrada manual** / **Ajustar** — correções pontuais
4. **Movimentações** — auditoria (`purchase_entry`, `initial_load`, …)

## Testes

```bash
cd api && go test ./internal/nfce/
```

## Fora deste slice

Visão OpenAI real para cupom, baixa por texto, receitas, voz, multi-usuário e lista de compras.
