import 'dart:io';

import 'package:path/path.dart' as path;
import 'package:path_provider/path_provider.dart';
import 'package:sembast/sembast_io.dart';

import '../domain/local_capture_store.dart';
import 'sembast_local_capture_store.dart';

Future<LocalCaptureStore> openLocalCaptureStore() async {
  final directory = await getApplicationDocumentsDirectory();
  await directory.create(recursive: true);
  final database = await databaseFactoryIo.openDatabase(
    path.join(directory.path, 'weavebrain_capture_v1.db'),
  );
  final store = SembastLocalCaptureStore(database);
  await store.recoverInterruptedSync();
  return store;
}

String get localCaptureSource {
  if (Platform.isAndroid) return 'mobile_android';
  if (Platform.isIOS) return 'mobile_ios';
  return 'desktop';
}
