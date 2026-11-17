# JUnit no CI — US-17 / issue #68

```bash
httpgo test tests/ --output junit --output-file target/test-results.xml
```

O formato exige destino em arquivo. Cria diretórios pais quando possível;
substitui o arquivo existente somente após obter resultados de execução.
stdout fica vazio, diagnósticos seguros vão a stderr. Cada arquivo é um testcase,
com nome normalizado relativo ao cwd (fallback absoluto entre unidades),
`classname="httpgo"` e duração em segundos. O diretório corrente e os caminhos
precisam permanecer estáveis entre execuções para preservar a identidade.

| Execução | XML | Exit code |
|---|---|---|
| Arquivo aprovado | `testcase` sem filho de falha | 0 |
| Status/header/JSON/body divergente | `failure` com diagnóstico localizado | 1 |
| Entrada, avaliação, captura, renderização ou transporte | `error` com diagnóstico seguro | 2 |
| Arquivo restante por fail-fast/cancelamento/falha de apresentação | `skipped`, duração zero | preserva a causa da interrupção |
| Criação, escrita curta, escrita ou fechamento do relatório falha | pode restar arquivo parcial; sem confirmação de publicação | 2 |
| Configuração/descoberta impede montar a execução | não cria nem modifica relatório | 2 |

Em suítes, **2 prevalece sobre 1**. Resultados falhos ainda produzem o artefato
quando gravável. O renderer não reenvia requests ou reavalia assertions. Nunca
use o mesmo destino de um arquivo de entrada; a escrita não é transacional.
Tempos e contagens são por arquivo, não por request ou tentativa de polling.
Não inclui `system-out`, bodies ou coleções de headers. Reutiliza a proteção de
segredos de ambiente/referências e nomes marcados com `--secret`; flags não são
fonte de credenciais. A proteção não detecta valores arbitrários não marcados ou
transformações desconhecidas. XML recebe a sanitização antes do escaping e após
a última descoberta de segredo; o terminal incremental não pode ser redigido
retroativamente quando um valor só é identificado como secreto mais tarde.
YAML e interpolação externa continuam extensões HTTPGo, não sintaxe Hurl.

## Consumidor de referência

[`.github/workflows/junit.yml`](../.github/workflows/junit.yml) usa
[`dorny/test-reporter`](https://github.com/dorny/test-reporter) **v2.6.0**, parser
`java-junit`, fixado no commit `3d76b34a4535afbd0600d347b09a6ee5deb3ed7f`:

1. Executa os testes do renderer e os E2E contra `httptest.Server` com binário real.
   Retém quatro XMLs: sucesso, misto (`failure` + `error`), fail-fast de assertion
   e fail-fast de transporte. Nomes contêm Unicode, aspas e caracteres reservados.
2. Compila o parser original do consumidor num checkout separado, sem alterá-lo;
   scripts em `tests/ci/` verificam ingestão, nomes, diagnósticos, tempos e estados.
3. Retém os artefatos via `upload-artifact` e os apresenta na interface de checks
   e no resumo do GitHub Actions usando o mesmo consumidor fixado.
4. Confere os totais publicados: **9 aprovados, 4 falhos/erros, 3 skipped**,
   16 testcases em quatro execuções independentes. A fixture do renderer é
   validada separadamente, não adicionada a esses totais.

O consumidor apresenta `failure` e `error` como **failed**; a distinção permanece
no XML. Diagnósticos `.hurl` localizados são texto, sem prometer links de source
Java para arquivos Hurl. Os negativos são deliberados: `fail-on-error: false`
permite apresentar o relatório de demonstração, mas os testes e as assertions de
contagem continuam obrigatórios. Isso não desabilita os gates do projeto nem é
recomendação de ignorar falhas na suíte real do usuário.

O workflow usa apenas `contents: read` e `checks: write`. Em PR de fork, o token
não pode criar checks: ingestão pelo parser e retenção de artefatos continuam
obrigatórias, mas a publicação do check é omitida. Não usa `pull_request_target`
para executar código de PR com permissões elevadas. Não há envio externo pelo
binário; a publicação é responsabilidade exclusiva do workflow/consumidor.

## Reprodução local da ingestão

Pré-requisitos de desenvolvimento: Go conforme `go.mod`, Git, Node 24 e npm.
Não são dependências de runtime do HTTPGo. Escolha diretórios temporários novos:

```bash
go test ./internal/report -count=1
go test ./tests/e2e -run '^TestJUnit' -count=1 -junit-artifact-dir=/tmp/httpgo-junit

git clone https://github.com/dorny/test-reporter.git /tmp/httpgo-junit-consumer
git -C /tmp/httpgo-junit-consumer checkout 3d76b34a4535afbd0600d347b09a6ee5deb3ed7f
(cd /tmp/httpgo-junit-consumer && npm ci --ignore-scripts && npm run build)
node tests/ci/junit-consumer.mjs /tmp/httpgo-junit-consumer testdata/report/junit.xml
node tests/ci/junit-execution.mjs /tmp/httpgo-junit-consumer /tmp/httpgo-junit
```

O golden `testdata/report/junit.xml` é gerado por `TestJUnitResults`; para uma
mudança intencional, `go test ./internal/report -run '^TestJUnitResults$' -update`
e revisão do diff. Não é uma observação de compatibilidade Hurl.

Exemplo mínimo executável: [scenario_junit.hurl](../testdata/scenarios/scenario_junit.hurl).
Com um servidor local retornando 200 em `/health`:

```bash
httpgo test testdata/scenarios/scenario_junit.hurl --var base_url=http://localhost:8080 --output junit --output-file target/test-results.xml
```

Não há dashboard próprio, histórico persistente, JSON/JUnit em stdout, formatos
adicionais ou publicação automática pelo binário. Consulte o
[contrato exato](subset.md#relatório-junit--us-17--issue-68) e a
[evidência executada](validation.md#us-17--issue-68--história-0017).
