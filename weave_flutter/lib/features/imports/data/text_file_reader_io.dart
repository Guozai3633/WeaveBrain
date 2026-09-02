/// 非 Web 平台的文本文件读取器。
///
/// MVP 未引入 file_picker 插件，App/桌面端请直接粘贴文本；Web 端支持
/// 原生文件选择（见 `text_file_reader_web.dart`）。
abstract interface class TextFileReader {
  /// 选择并读取一个 UTF-8 文本文件。无法选择时返回 null（调用方提示粘贴）。
  Future<String?> pickAndReadUtf8();
}

class IoTextFileReader implements TextFileReader {
  const IoTextFileReader();

  @override
  Future<String?> pickAndReadUtf8() async => null;
}

TextFileReader createTextFileReader() => const IoTextFileReader();
