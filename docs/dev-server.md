# Servidor dummy de desenvolvimento

`tools/dummy/main.go` é um utilitário **exclusivamente manual** de desenvolvimento,
separado do produto e da suíte de testes. Usa apenas biblioteca padrão; não é uma
dependência de runtime. A tag `//go:build ignore` o exclui da descoberta de pacotes
para build, lint e testes automáticos. O Make executa o arquivo explicitamente,
sem incluir o utilitário no binário httpgo.

## Iniciar e encerrar

Na raiz do repositório:

```sh
make serve
# Ou o alias:
make dummy
# Sem Make, inclusive no Windows:
go run ./tools/dummy/main.go
```

O processo fica em primeiro plano e escuta **somente em `127.0.0.1:8080`**.
Encerre com **Ctrl+C**: o servidor fecha o listener e permite até cinco segundos
para requisições em andamento terminarem; depois força o fechamento.

A porta 8080 deve estar livre. Se já estiver ocupada, o comando falha com um
diagnóstico; não encerra outro processo nem escolhe outra porta silenciosamente.
O servidor não é iniciado automaticamente em `make build`, `make test` ou E2E.

## Endpoints

Todas as respostas usam `Content-Type: application/json`.

| Método | Caminho | Status | Body |
|---|---|---|---|
| GET | `/health` | 200 | `{"status":"ok"}` |
| GET | `/inexistente` | 404 | `{"error":"not found"}` |
| GET | `/error` | 500 | `{"error":"dummy server error"}` |
| GET | Qualquer outro caminho | 404 | `{"error":"not found"}` |
| Outros métodos | Qualquer caminho | 405 | `{"error":"method not allowed"}`, header `Allow: GET` |

Queries não modificam os resultados. O servidor não ecoa query, credenciais,
headers ou bodies, nem mantém log de requisições. Não há TLS, autenticação,
configuração externa ou promessa de uso em produção.

## Executar os cenários

Com o servidor aberto em um terminal, use outro:

```sh
make build
bin/httpgo test testdata/scenarios/scenario_get_status.hurl
bin/httpgo test testdata/scenarios/scenario_unvalidated_status.hurl
```

O primeiro cenário valida 200 e 404; o segundo recebe 500, aceita a resposta e
informa **sem validação de status**. Ambos terminam com código 0.

## Fora da suíte automatizada

Não há testes do dummy integrados ao projeto. `go test ./...`, `go build ./...`
e `make lint` não carregam esse arquivo. O servidor é iniciado somente por ação
manual, nunca por build, testes, E2E ou CI. Os testes existentes continuam com
seus próprios servidores `httptest`, sem reutilizar ou depender deste utilitário.
