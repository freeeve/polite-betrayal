import 'package:flutter/material.dart';

import '../../core/map/province_data.dart';

/// Autocomplete input for selecting a Diplomacy province.
///
/// Matches on province name and ID (case-insensitive).
/// Calls [onSelected] with the lowercase province name.
class ProvinceAutocomplete extends StatelessWidget {
  final String labelText;
  final ValueChanged<String> onSelected;
  final List<Province> suggestions;

  const ProvinceAutocomplete({
    super.key,
    required this.labelText,
    required this.onSelected,
    required this.suggestions,
  });

  @override
  Widget build(BuildContext context) {
    return Autocomplete<Province>(
      optionsBuilder: (textEditingValue) {
        final query = textEditingValue.text.toLowerCase();
        if (query.isEmpty) return suggestions;
        return suggestions.where((p) =>
            p.name.toLowerCase().contains(query) ||
            p.id.startsWith(query));
      },
      displayStringForOption: (p) =>
          p.isSupplyCenter ? '${p.name} (SC)' : p.name,
      onSelected: (p) => onSelected(p.name.toLowerCase()),
      fieldViewBuilder: (context, controller, focusNode, onFieldSubmitted) {
        return TextField(
          controller: controller,
          focusNode: focusNode,
          decoration: InputDecoration(
            labelText: labelText,
            border: const OutlineInputBorder(),
            isDense: true,
          ),
          onSubmitted: (_) => onFieldSubmitted(),
        );
      },
      optionsViewBuilder: (context, onSelected, options) {
        return Align(
          alignment: Alignment.topLeft,
          child: Material(
            elevation: 4,
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxHeight: 200, maxWidth: 300),
              child: ListView.builder(
                padding: EdgeInsets.zero,
                shrinkWrap: true,
                itemCount: options.length,
                itemBuilder: (context, index) {
                  final p = options.elementAt(index);
                  return ListTile(
                    dense: true,
                    title: Text(p.isSupplyCenter ? '${p.name} (SC)' : p.name),
                    subtitle: Text(p.id),
                    onTap: () => onSelected(p),
                  );
                },
              ),
            ),
          ),
        );
      },
    );
  }
}
