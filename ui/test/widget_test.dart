import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:polite_betrayal/main.dart';

void main() {
  testWidgets('App starts and shows login screen', (WidgetTester tester) async {
    await tester.pumpWidget(
      const ProviderScope(child: PoliteBetrayal()),
    );
    await tester.pumpAndSettle();

    expect(find.text('Polite Betrayal'), findsOneWidget);
    expect(find.text('Dev Login'), findsOneWidget);
  });
}
