Native CPU runtime files are generated here by `python3 build/prepare-runtime.py`.
Only the current macOS/Linux architecture is embedded. Archives are downloaded
from Microsoft's pinned ONNX Runtime release and checked against the SHA-256
values in `build/native-runtimes.json`. End users do not download runtimes.
