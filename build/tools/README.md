# Windows 安装包编译工具

`nsis-3.11.zip` 是未经修改的 NSIS 3.11 官方便携编译工具归档，供构建脚本解包使用，避免云端构建受下载镜像的 HTML 页面或网络限制影响。该文件不包含 SmartNote 构建产物。

- 上游：https://nsis.sourceforge.io/
- 下载来源：https://downloads.sourceforge.net/project/nsis/NSIS%203/3.11/nsis-3.11.zip
- 大小：2,361,546 字节。
- SHA-256：`c7d27f780ddb6cffb4730138cd1591e841f4b7edb155856901cdf5f214394fa1`。
- 编译器 `makensis.exe` SHA-256：`f497e92deb9f179f7b8f7553fcb3bd04f511bb2949e5e4a2aee80a10f7b20431`。
- 许可：归档内的许可文件与 [NSIS-LICENSE.txt](../windows/NSIS-LICENSE.txt)。

`build/Build-Installer.ps1` 在使用前同时校验归档与编译器，不通过则停止。构建器优先复用校验一致的已安装工具，其次使用此归档；未携带此归档的精简源码分发仍可尝试官方镜像下载。
