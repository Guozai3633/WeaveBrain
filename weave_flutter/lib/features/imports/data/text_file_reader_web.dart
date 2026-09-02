// ignore_for_file: avoid_web_libraries_in_flutter, deprecated_member_use

import 'dart:async';
import 'dart:html' as html;

/// Web 平台的文本文件读取器：通过原生 `<input type=file>` 选择文件，
/// 用 FileReader 按 UTF-8 读取文本。
abstract interface class TextFileReader {
  /// 选择并读取一个 UTF-8 文本文件。用户取消时返回 null。
  Future<String?> pickAndReadUtf8();
}

class WebTextFileReader implements TextFileReader {
  const WebTextFileReader();

  @override
  Future<String?> pickAndReadUtf8() async {
    final completer = Completer<String?>();
    final input = html.FileUploadInputElement()
      ..accept = '.txt,.md,.csv,.jsonl,text/plain,text/csv,text/markdown'
      ..click();
    input.onChange.first.then((_) {
      final files = input.files;
      final file = (files != null && files.isNotEmpty) ? files.first : null;
      if (file == null) {
        completer.complete(null);
        return;
      }
      final reader = html.FileReader();
      reader.onLoad.first.then((_) {
        completer.complete(reader.result as String?);
      });
      reader.onError.first.then((_) => completer.complete(null));
      reader.readAsText(file, 'utf-8');
    });
    return completer.future;
  }
}

TextFileReader createTextFileReader() => const WebTextFileReader();
