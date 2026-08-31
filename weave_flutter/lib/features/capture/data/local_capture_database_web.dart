import 'package:sembast_web/sembast_web.dart';

import '../domain/local_capture_store.dart';
import 'sembast_local_capture_store.dart';

Future<LocalCaptureStore> openLocalCaptureStore() async {
  final database = await databaseFactoryWeb.openDatabase(
    'weavebrain_capture_v1',
  );
  final store = SembastLocalCaptureStore(database);
  await store.recoverInterruptedSync();
  return store;
}

String get localCaptureSource => 'web';
