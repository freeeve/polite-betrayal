import 'package:flutter/material.dart';

/// Bottom sheet for selecting a coast on split-coast provinces (spa/stp/bul).
class CoastPicker extends StatelessWidget {
  final String province;
  final List<String> coasts;
  final void Function(String coast) onSelect;

  const CoastPicker({
    super.key,
    required this.province,
    required this.coasts,
    required this.onSelect,
  });

  static void show(BuildContext context, {
    required String province,
    required List<String> coasts,
    required void Function(String coast) onSelect,
  }) {
    showModalBottomSheet(
      context: context,
      builder: (_) => CoastPicker(
        province: province,
        coasts: coasts,
        onSelect: (coast) {
          Navigator.of(context).pop();
          onSelect(coast);
        },
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(24),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(
            'Select coast for ${_provinceName(province)}',
            style: Theme.of(context).textTheme.titleMedium,
          ),
          const SizedBox(height: 16),
          ...coasts.map((coast) => ListTile(
                leading: const Icon(Icons.navigation),
                title: Text(_coastLabel(coast)),
                onTap: () => onSelect(coast),
              )),
        ],
      ),
    );
  }

  String _provinceName(String id) {
    return switch (id) {
      'spa' => 'Spain',
      'stp' => 'St. Petersburg',
      'bul' => 'Bulgaria',
      _ => id,
    };
  }

  String _coastLabel(String coast) {
    return switch (coast) {
      'nc' => 'North Coast',
      'sc' => 'South Coast',
      'ec' => 'East Coast',
      _ => coast,
    };
  }
}
