# Protocol

Protocol Buffers (proto3) definitions live in `protocol/icehive/v1/`. Code generation is managed by [Buf](https://buf.build/).

## Regenerating stubs

```bash
make -C protocol
```

This runs `buf generate` which writes Go + Connect stubs into `services/common/pkg/gen/` and TypeScript (Protobuf-ES) into `frontend/src/gen/` for the web UI.
