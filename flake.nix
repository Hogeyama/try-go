{
  description = "Sample Haskell project";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/8759b61c615281c726d67f5e61cc81601be8e042";
    flake-parts.url = "github:hercules-ci/flake-parts";
    flake-compat.url = "github:edolstra/flake-compat";
    flake-compat.flake = false;
    nix-bundle-elf.url = "github:Hogeyama/nix-bundle-elf/main";
    nix-bundle-elf.inputs.nixpkgs.follows = "nixpkgs";
    flake-root.url = "github:srid/flake-root";
    devshell.url = "github:numtide/devshell";
    process-compose-flake.url = "github:Platonic-Systems/process-compose-flake";
  };

  outputs =
    inputs@{ self, flake-parts, ... }:
    let
      postgres_port = 5432;
      outputs-overlay = pkgs: prev: {
        # TODO
      };
    in
    flake-parts.lib.mkFlake { inherit inputs; } {
      imports = [
        inputs.flake-root.flakeModule
        inputs.devshell.flakeModule
        inputs.process-compose-flake.flakeModule
      ];
      systems = [
        "x86_64-linux"
        "aarch64-linux"
      ];
      perSystem =
        { config
        , lib
        , self'
        , inputs'
        , pkgs
        , system
        , ...
        }:
        {
          _module.args.pkgs = import inputs.nixpkgs {
            inherit system;
            overlays = [ outputs-overlay ];
          };

          packages = { };

          devshells.default = {
            packagesFrom = [
            ];
            packages = [
              pkgs.go
              pkgs.go-swag
              pkgs.oapi-codegen
              pkgs.sqlc
              pkgs.goose
              pkgs.golangci-lint
            ];
            commands = [
              {
                name = "lint";
                help = "Run linters";
                command = "golangci-lint run";
              }
              {
                name = "run-server";
                help = "Run postgres and backend server";
                command = ''
                  cd "$PRJ_ROOT"
                  nix run .#processes -- -n postgres -n server "$@"
                '';
              }
              {
                name = "run-postgres";
                help = "Run postgres";
                command = ''
                  cd "$PRJ_ROOT"
                  nix run .#processes -- -n postgres "$@"
                '';
              }
              {
                name = "gen-openapi";
                help = "Generate Go code from OpenAPI spec";
                command = ''
                  cd "$PRJ_ROOT"
                  mapfile -t tags < <(yq -r '
                    .paths|to_entries[].value|to_entries[].value.tags|select(.)[]
                  ' <openapi.yaml | sort | uniq)
                  for tag in "''${tags[@]}"; do
                    oapi-codegen \
                      -generate types,gin,strict-server \
                      -package "''${tag}http" \
                      -include-tags "$tag" \
                      -o "internal/$tag/http/handlers_def.go" \
                      openapi.yaml
                  done
                '';
              }
              {
                name = "gen-sqlc";
                help = "Generate Go code via SQLC";
                command = ''
                  cd "$PRJ_ROOT"
                  sqlc generate
                '';
              }
              {
                name = "migrate";
                help = "Run goose migrations";
                command = ''
                  cd "$PRJ_ROOT"
                  target=$(
                    find -type d | grep 'db/migrations$' |
                    ${pkgs.fzf}/bin/fzf --prompt 'Select migration directory: '
                  )
                  if [[ -z "$target" ]]; then
                    echo "No migration directory selected"
                    exit 1
                  fi
                  if [[ ! -e "$target" ]]; then
                    echo "Migration directory does not exist: $target"
                    exit 1
                  fi
                  if [[ -z "$*" ]]; then
                    echo "No goose command specified."
                    if command -v xclip 2>&1 >/dev/null; then
                      echo -n "goose -dir "$target" postgres \"\$DATABASE_URL\"" |
                        xclip -selection clipboard
                      echo "Command copied to clipboard."
                    fi
                    if command -v xsel 2>&1 >/dev/null; then
                      echo -n "goose -dir "$target" postgres \"\$DATABASE_URL\"" |
                        xsel -b
                      echo "Command copied to clipboard."
                    fi
                    exit 1
                  else
                    goose -dir "$target" postgres "$DATABASE_URL" "$@"
                  fi
                '';
              }
            ];
            env = [
            ];
          };

          legacyPackages = pkgs;

          process-compose =
            let
              postgres =
                let
                  get_pgdata = pkgs.writeShellApplication {
                    name = "get_pgdata";
                    text = ''
                      ROOT=$(${lib.getExe config.flake-root.package} 2>/dev/null || true)
                      PGDATA=''${ROOT:-"$PWD"}/pgdata
                      echo "$PGDATA"
                    '';
                  };
                in
                {
                  namespace = "postgres";
                  command = pkgs.writeShellApplication {
                    name = "postgres";
                    runtimeInputs = [ pkgs.postgresql_16 ];
                    text = ''
                      set -e
                      PGDATA=$(${lib.getExe get_pgdata})
                      if ! [[ -e "$PGDATA/PG_VERSION" ]]; then
                          mkdir -p "$PGDATA"
                          initdb -U postgres -D "$PGDATA" -A trust
                      fi
                      postgres \
                        -D "$PGDATA" \
                        -k "$PGDATA" \
                        -c config_file="$PRJ_ROOT/dev/postgresql.conf" \
                        -p ${builtins.toString postgres_port}
                    '';
                  };
                  readiness_probe = {
                    period_seconds = 1;
                    exec = {
                      command = "${lib.getExe (
                        pkgs.writeShellApplication {
                          name = "pg_isready";
                          runtimeInputs = [ pkgs.postgresql_16 ];
                          text = ''
                            PGDATA=$(${lib.getExe get_pgdata})
                            pg_isready --host "$PGDATA" -U postgres
                          '';
                        }
                      )}";
                    };
                  };
                };
              dev-front = {
                namespace = "dev-front";
                command = ''
                  cd "$PRJ_ROOT/frontend" && pnpm run dev
                '';
              };
              server = {
                namespace = "server";
                command = ''
                  cd "$PRJ_ROOT" && go run main.go
                '';
                depends_on."postgres".condition = "process_healthy";
              };
            in
            {
              type = "";
              cli = {
                options = {
                  no-server = true;
                };
              };
              processes = {
                settings.processes = {
                  inherit postgres server;
                };
              };
            };
        };
    };
}

