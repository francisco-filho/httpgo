# Subconjunto suportado — US-01 (história #4)

Este documento define somente o que o binário implementa nesta história.
Não implica suporte ao formato Hurl completo.

## Gramática

Um arquivo UTF-8 contém ao menos um `GET <URL absoluta HTTP/HTTPS>`.
A expectativa opcional `HTTP <status>` associa-se ao GET anterior, uma única vez.
O status tem exatamente três dígitos ASCII, entre 100 e 599.
São aceitos espaços/tabs de indentação, linhas vazias, LF/CRLF e comentários em
linha própria iniciados por `#`. Comentários e linhas vazias não desfazem a
associação entre GET e HTTP. O GET seguinte inicia outra entrada.

```hurl
# A expectativa pertence ao GET anterior.
GET https://example.test/health
HTTP 200

# Sem expectativa: executa, mas NÃO valida status.
GET https://example.test/error
```

URLs são verificadas estruturalmente antes da rede: esquema HTTP/HTTPS, host não
vazio, IPv6 entre colchetes (inclusive zona percent-encoded) e porta numérica de
0 a 65535 quando declarada. Isso não comprova resolução DNS, conexão ou
existência do endpoint. Scheme é normalizado sem distinção de caixa.

O parser valida **todo** o arquivo antes de retornar um cenário executável.
Arquivo vazio/com só comentários, URL inválida, status inválido, expectativa
órfã/duplicada e instrução desconhecida são erros de entrada. Nada do arquivo é
enviado nesses casos, mesmo quando o erro aparece depois de GETs válidos.

Construções reconhecidas e não suportadas são diagnosticadas separadamente de
sintaxe inválida. Não são aceitos: outros métodos, headers/bodies do usuário,
variáveis, `[Captures]`, `[Asserts]` e demais seções, `file,...;`, versões como
`HTTP/1.1` na expectativa ou comentários após uma instrução. O texto bruto
não é ecoado. O corpus negativo está em `testdata/scenarios/unsupported_*.hurl`.

## Execução HTTP

- Cada GET é iniciado em ordem. Não há paralelismo ou retry, inclusive retry
  implícito do transporte. HTTP/1.1 sem conexão persistente evita o replay de
  requisições em conexão reutilizada. HTTP/2 não é habilitado nesta história.
- Timeout total **fixo de 10 segundos** por GET, contado uma vez e compartilhado
  por conexão/TLS, redirects e consumo de todos os bodies. Não há flag de ajuste.
- TLS é verificado; não há opção para ignorar certificados.
- Redirects 301/302/303/307/308 com Location são seguidos até **10 saltos**.
  O 11º salto é bloqueado antes de enviar ao próximo destino. Resposta sem
  Location não causa redirect e seu status é o resultado.
- Apenas a mesma origem do GET inicial: scheme e hostname sem distinção de caixa,
  porta efetiva numérica (80 para HTTP e 443 para HTTPS, quando omitida).
  Não se usa DNS para equiparar hosts: localhost não equivale a 127.0.0.1.
  Mudar scheme/host/porta é erro de transporte, sem contato ao destino proibido.
- Bodies são consumidos em streaming e fechados antes de avaliar o status final.
  Resposta incompleta, erro de leitura/fechamento ou timeout no body não aprova
  a expectativa só porque os headers já chegaram. Não há body em relatório.
- Sem expectativa, inclusive 4xx/5xx é sucesso **sem validação de status**.
  Com expectativa, a comparação é de igualdade com o status final recebido.
- Divergência, falha de transporte ou cancelamento interrompe as próximas entradas.
  Ctrl+C cancela a requisição em andamento. Evidência de sinal real: Linux;
  macOS/Windows ainda não têm aceite nativo (ver `validation.md`).
- Não há cookie jar, proxy configurável, configuração YAML ou leitura automática
  de HTTPGO_*. O transporte não lê HTTP_PROXY/HTTPS_PROXY nesta história.

## CLI e saída

```text
httpgo test <arquivo>
httpgo --help
httpgo test --help
httpgo --version
```

`test` exige exatamente um arquivo regular legível. Não aceita diretórios,
múltiplos arquivos, entrada de cenário/segredos por stdin nem flags adicionais.
`--output`, `--output-file`, `--var`, `--secret`, `check` e configurações futuras
não estão implementados ou anunciados como disponíveis.

Cada entrada iniciada tem uma linha de resultado com arquivo/linha/coluna,
URL segura e uma classificação: `status validado`, `sem validação de status`,
`status divergente` ou `erro de transporte`. Divergências mostram esperado e
recebido. Falha de transporte não inventa status.

O resumo em stdout informa:

- `total`: entradas validadas pelo parser;
- `iniciadas`: chamadas ao executor — **não comprova recebimento pelo servidor**;
- `concluídas`: respostas totalmente consumidas/fechadas, inclusive divergentes;
- `validadas`: comparações de status aprovadas;
- `sem_validação`: respostas concluídas sem expectativa;
- `não_iniciadas`: entradas não iniciadas por interrupção.

Erros de entrada anteriores ao runner não apresentam resumo de execução.
Diagnósticos ficam em stderr e incluem posição quando identificável. Erros
brutos da rede, Location, linha de entrada e bodies nunca são impressos.

| Exit code | Significado |
|---|---|
| 0 | todas as entradas concluídas sem divergência; inclui sem expectativa |
| 1 | primeira divergência de status |
| 2 | uso, arquivo, parsing, transporte, TLS, timeout, cancelamento ou escrita |

Erro ao escrever stdout/stderr também é categoria 2; não transforma divergência
em sucesso. A apresentação usa sempre texto simples sem cores/animações,
independentemente de TTY e NO_COLOR: não há detecção de terminal nem formato
alternativo dependente do ambiente. O E2E comprova pipe e PTY Linux real.

`--version` mostra versão, commit e data. Para defini-los no build:

```sh
go build -ldflags '-X main.version=0.1.0 -X main.commit=abc123 -X main.date=2026-11-14' -o bin/httpgo ./cmd/httpgo
```

Sem esses valores, mostra `dev`, `unknown`, `unknown`.

## Segurança e limites

A URL exibível remove userinfo (usuário e senha), **oculta toda a query**, inclusive
seus nomes, e não exibe fragmento. A URL operacional não é mutilada: credenciais e
query continuam disponíveis para a requisição. URL inválida recebe diagnóstico
controlado sem expor o token malformado. O caminho do arquivo é exibido com
quoting para escapar caracteres de controle.

**O path da URL permanece visível: não coloque segredos nele.** Não é possível
detectar qualquer segredo arbitrário embutido no path/hostname/nome de arquivo,
nem proteger o conteúdo do arquivo em disco ou argumentos no histórico/processos.
Esta sanitização não é um mecanismo de entrada segura de segredos por flags.

## Diferenças em relação ao Hurl

- O subconjunto é menor: comentários inline, respostas com versão e outros
  métodos/seções são rejeitados, não ignorados.
- Códigos de saída, resultados/resumo e mensagens pertencem ao contrato httpgo,
  não ao Hurl. Ex.: divergência é 1 aqui e 4 no Hurl 8.0.0 usado no corpus.
- Redirects são automáticos mas limitados à mesma origem; o diferencial usa
  `--location --max-redirs 10 --max-time 10 --retry 0` no Hurl para aproximar as
  políticas compartilhadas. Essa aproximação não torna os motores equivalentes.
- Configuração YAML hierárquica e interpolação externa, quando implementadas,
  serão **extensões httpgo**, não parte do Hurl; ambas continuam fora desta história.

Veja a matriz e evidências limitadas ao corpus em `compatibility.md`.
