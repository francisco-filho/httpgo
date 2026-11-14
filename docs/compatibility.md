# Matriz de compatibilidade — US-01

## Escopo da evidência

Referência fixada: **Hurl 8.0.0**. O diferencial executa o binário, não importa nem
lê seu código-fonte. `testdata/compatibility/` é contrato versionado: cada `.hurl`
tem um `.json` com classificação, exit code e paths observados em cada ferramenta.
Alterar esses arquivos em entregas futuras é mudança de contrato a sinalizar no PR.

```sh
make compatibility
# Atualizar observações intencionalmente (sempre revisar o diff):
go test -tags compatibility ./tests/e2e -run '^TestCompatibility$' -count=1 -args -update
```

Ausência de Hurl ou versão diferente de 8.0.0 causa **falha**, não skip. O alvo
não baixa ou instala Hurl automaticamente. O teste substitui a origem absoluta
`http://example.invalid` por um httptest.Server local em cópia temporária. Isso
é adaptação do harness, **não suporte a variáveis/interpolação no httpgo**.

Os comandos Hurl usam `--no-color --no-output --location --max-redirs 10
--max-time 10 --retry 0 --no-cookie-store --no-proxy '*'`. Não se compara texto
bruto de saída nem assume igualdade de códigos: a comparação observa aceitação,
falha, ordem e contatos efetivos, conforme cada par esperado. Os testes próprios
httpgo verificam conteúdo informativo e sanitização em ambos os streams.

## Matriz observada (Linux amd64)

| Corpus | httpgo | Hurl 8.0.0 | Classificação |
|---|---|---|---|
| `sequence` | 0; /health, /missing | 0; mesmos paths | compartilhado: GETs, comentário próprio, linhas vazias, 200/404 |
| `boundary` | 0; /boundary | 0; mesmo path | compartilhado: status 599 |
| `unvalidated` | 0; /error retorna 500 | 0; mesmo path | compartilhado: sem expectativa aceita 500 |
| `mismatch` | 1; somente /unavailable | 4; mesmo path | semântica de divergência/fail-fast compartilhada; exit codes diferentes |
| `redirect` | 0; /redirect, /health | 0; mesmos paths com flags alinhadas | compartilhado: status final validado |
| `inline_comment` | 2; zero envios | 0; /health | restrição intencional: comentário inline rejeitado |
| `http_version` | 2; zero envios | 0; /health | restrição intencional: resposta versionada rejeitada |
| `post` | 2; zero envios | 0; /health | restrição intencional: somente GET nesta história |
| `cross_origin` | 2; somente /cross | 0; /cross, /health | política intencional: localhost difere de 127.0.0.1, mesmo que resolva igual |

Estas observações **não** anunciam compatibilidade geral com Hurl ou equivalência
dos motores net/http e libcurl. Timeout, certificados, pooling, headers implícitos,
DNS e diagnósticos podem diferir. As demais garantias httpgo são verificadas nos
testes próprios. Headers/bodies, variáveis, capturas, seções e demais construções
fora de escopo continuam explicitamente não suportadas.

## Extensões e restrições

YAML hierárquico e interpolação em arquivos externos são extensões futuras do
httpgo, **não capacidades do formato Hurl implementadas nesta história**.
Não há configuração YAML ou interpolação nesta entrega. A gramática menor,
saída/exit codes próprios e restrição de redirects estão detalhados em `subset.md`.

## Ambiente usado

A execução local usou Hurl 8.0.0 com libcurl 8.18.0. O binário da sandbox emite um
aviso de versionamento de libcurl em stderr ao iniciar; o harness não o confunde
com falha de HTTP ou altera os resultados para escondê-lo. A matriz acima passou
contra esse binário. Evidência de portabilidade e gate pendente: `validation.md`.
