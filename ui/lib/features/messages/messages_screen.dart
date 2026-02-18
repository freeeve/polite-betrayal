import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/auth/auth_notifier.dart';
import '../../core/map/adjacency_data.dart';
import '../../core/map/province_data.dart';
import '../../core/models/game_state.dart';
import '../../core/theme/app_theme.dart';
import '../game/game_notifier.dart';
import 'message_bubble.dart';
import 'messages_notifier.dart';
import 'province_autocomplete.dart';

class MessagesScreen extends ConsumerStatefulWidget {
  final String gameId;
  const MessagesScreen({super.key, required this.gameId});

  @override
  ConsumerState<MessagesScreen> createState() => _MessagesScreenState();
}

class _MessagesScreenState extends ConsumerState<MessagesScreen>
    with SingleTickerProviderStateMixin {
  late final TabController _tabController;
  final _inputController = TextEditingController();
  String? _selectedRecipientPower;

  @override
  void initState() {
    super.initState();
    // Tabs: Public + 6 other powers
    _tabController = TabController(length: 7, vsync: this);
    _tabController.addListener(() {
      setState(() {
        if (_tabController.index == 0) {
          _selectedRecipientPower = null;
        } else {
          _selectedRecipientPower = allPowers[_tabController.index - 1];
        }
      });
    });
  }

  @override
  void dispose() {
    _tabController.dispose();
    _inputController.dispose();
    super.dispose();
  }

  Future<void> _send() async {
    final content = _inputController.text.trim();
    if (content.isEmpty) return;

    // For DMs, find the user ID of the recipient power.
    String? recipientId;
    if (_selectedRecipientPower != null) {
      final game = ref.read(gameProvider(widget.gameId));
      final recipient = game.game?.players
          .where((p) => p.power == _selectedRecipientPower)
          .firstOrNull;
      recipientId = recipient?.userId;
    }

    final ok = await ref
        .read(messagesProvider(widget.gameId).notifier)
        .sendMessage(content, recipientId: recipientId);

    if (ok) {
      _inputController.clear();
    } else if (mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Failed to send message')),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final msgState = ref.watch(messagesProvider(widget.gameId));
    final auth = ref.watch(authProvider);
    final userId = auth.user?.id ?? '';
    final gameState = ref.watch(gameProvider(widget.gameId));
    final myPower = gameState.powerForUser(userId);

    // Build power-to-userId map.
    final powerToUserId = <String, String>{};
    final userIdToPower = <String, String>{};
    for (final player in gameState.game?.players ?? []) {
      if (player.power != null && player.power!.isNotEmpty) {
        powerToUserId[player.power!] = player.userId;
        userIdToPower[player.userId] = player.power!;
      }
    }

    return Scaffold(
      appBar: AppBar(
        title: const Text('Diplomacy'),
        bottom: TabBar(
          controller: _tabController,
          isScrollable: true,
          tabs: [
            const Tab(text: 'Public'),
            ...allPowers.where((p) => p != myPower).take(6).map((p) => Tab(
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Container(
                        width: 12,
                        height: 12,
                        decoration: BoxDecoration(
                          color: PowerColors.forPower(p),
                          shape: BoxShape.circle,
                        ),
                      ),
                      const SizedBox(width: 4),
                      Text(PowerColors.label(p)),
                    ],
                  ),
                )),
          ],
        ),
      ),
      body: Column(
        children: [
          Expanded(
            child: msgState.loading
                ? const Center(child: CircularProgressIndicator())
                : TabBarView(
                    controller: _tabController,
                    children: [
                      // Public messages
                      _MessageList(
                        messages: msgState.messages
                            .where((m) => m.isPublic)
                            .toList(),
                        userId: userId,
                        userIdToPower: userIdToPower,
                      ),
                      // Per-power DMs
                      ...allPowers.where((p) => p != myPower).take(6).map((p) {
                        final recipientUserId = powerToUserId[p];
                        return _MessageList(
                          messages: msgState.messages.where((m) {
                            if (m.isPublic) return false;
                            return (m.senderId == userId &&
                                    m.recipientId == recipientUserId) ||
                                (m.senderId == recipientUserId &&
                                    m.recipientId == userId);
                          }).toList(),
                          userId: userId,
                          userIdToPower: userIdToPower,
                        );
                      }),
                    ],
                  ),
          ),
          _MessageInput(
            controller: _inputController,
            onSend: _send,
            isPublic: _tabController.index == 0,
            gameState: gameState.gameState,
            myPower: myPower,
            recipientPower: _selectedRecipientPower,
          ),
        ],
      ),
    );
  }
}

class _MessageList extends StatelessWidget {
  final List<dynamic> messages;
  final String userId;
  final Map<String, String> userIdToPower;

  const _MessageList({
    required this.messages,
    required this.userId,
    required this.userIdToPower,
  });

  @override
  Widget build(BuildContext context) {
    if (messages.isEmpty) {
      return const Center(child: Text('No messages yet'));
    }
    return ListView.builder(
      reverse: true,
      padding: const EdgeInsets.symmetric(vertical: 8),
      itemCount: messages.length,
      itemBuilder: (context, i) {
        final msg = messages[messages.length - 1 - i];
        return MessageBubble(
          message: msg,
          senderPower: userIdToPower[msg.senderId],
          isMe: msg.senderId == userId,
        );
      },
    );
  }
}

class _MessageInput extends StatelessWidget {
  final TextEditingController controller;
  final VoidCallback onSend;
  final bool isPublic;
  final GameState? gameState;
  final String? myPower;
  final String? recipientPower;

  const _MessageInput({
    required this.controller,
    required this.onSend,
    required this.isPublic,
    this.gameState,
    this.myPower,
    this.recipientPower,
  });

  void _showCannedMessages(BuildContext context) {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      builder: (ctx) => _CannedMessagePicker(
        onSelect: (msg) {
          controller.text = msg;
          Navigator.of(ctx).pop();
        },
        gameState: gameState,
        myPower: myPower,
        recipientPower: recipientPower,
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(8),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surfaceContainer,
        border: Border(top: BorderSide(color: Theme.of(context).dividerColor)),
      ),
      child: Row(
        children: [
          IconButton(
            onPressed: () => _showCannedMessages(context),
            icon: const Icon(Icons.flash_on),
            tooltip: 'Quick message',
          ),
          Expanded(
            child: TextField(
              controller: controller,
              decoration: InputDecoration(
                hintText: isPublic ? 'Public message...' : 'Private message...',
                border: const OutlineInputBorder(),
                contentPadding:
                    const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
              ),
              textInputAction: TextInputAction.send,
              onSubmitted: (_) => onSend(),
            ),
          ),
          const SizedBox(width: 8),
          IconButton.filled(
            onPressed: onSend,
            icon: const Icon(Icons.send),
          ),
        ],
      ),
    );
  }
}

/// Bottom sheet for picking canned diplomatic messages.
class _CannedMessagePicker extends StatefulWidget {
  final ValueChanged<String> onSelect;
  final GameState? gameState;
  final String? myPower;
  final String? recipientPower;

  const _CannedMessagePicker({
    required this.onSelect,
    this.gameState,
    this.myPower,
    this.recipientPower,
  });

  @override
  State<_CannedMessagePicker> createState() => _CannedMessagePickerState();
}

class _CannedMessagePickerState extends State<_CannedMessagePicker> {
  String? _selectedTemplate;
  String _province1 = '';
  String _province2 = '';
  String _power = '';

  static const _templates = [
    _CannedTemplate(
      label: 'Request support',
      template: 'Request support from {from} to {to}',
      fields: ['from', 'to'],
    ),
    _CannedTemplate(
      label: 'Non-aggression pact',
      template: "Please don't attack {province}, I won't attack yours",
      fields: ['province'],
    ),
    _CannedTemplate(
      label: 'Propose alliance',
      template: "Let's work together against {power}",
      fields: ['power'],
    ),
    _CannedTemplate(
      label: 'Threaten',
      template: "I'm coming for {province} — back off",
      fields: ['province'],
    ),
    _CannedTemplate(
      label: 'Offer deal',
      template: 'Deal: I take {mine}, you take {yours}',
      fields: ['mine', 'yours'],
    ),
    _CannedTemplate(label: 'Agree', template: 'Agreed', fields: []),
    _CannedTemplate(label: 'Reject', template: 'No deal', fields: []),
  ];

  /// Looks up a province ID from its lowercase name.
  String? _provinceIdFromName(String name) {
    if (name.isEmpty) return null;
    for (final entry in provinces.entries) {
      if (entry.value.name.toLowerCase() == name) return entry.key;
    }
    return null;
  }

  String _buildMessage(_CannedTemplate t) {
    var msg = t.template;
    if (t.fields.contains('from')) msg = msg.replaceAll('{from}', _province1);
    if (t.fields.contains('to')) msg = msg.replaceAll('{to}', _province2);
    if (t.fields.contains('province')) {
      msg = msg.replaceAll('{province}', _province1);
    }
    if (t.fields.contains('power')) msg = msg.replaceAll('{power}', _power);
    if (t.fields.contains('mine')) msg = msg.replaceAll('{mine}', _province1);
    if (t.fields.contains('yours')) msg = msg.replaceAll('{yours}', _province2);
    return msg;
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(16),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text('Quick Message',
              style: Theme.of(context).textTheme.titleMedium),
          const SizedBox(height: 12),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: _templates.map((t) {
              final selected = _selectedTemplate == t.label;
              return ChoiceChip(
                label: Text(t.label),
                selected: selected,
                onSelected: (_) {
                  setState(() {
                    _selectedTemplate = selected ? null : t.label;
                    if (t.fields.isEmpty && !selected) {
                      widget.onSelect(t.template);
                    }
                  });
                },
              );
            }).toList(),
          ),
          if (_selectedTemplate != null) ...[
            const SizedBox(height: 12),
            ..._buildFieldInputs(),
            const SizedBox(height: 8),
            FilledButton(
              onPressed: _canSend() ? _send : null,
              child: const Text('Send'),
            ),
          ],
        ],
      ),
    );
  }

  List<Widget> _buildFieldInputs() {
    final t = _templates.firstWhere((t) => t.label == _selectedTemplate);
    final widgets = <Widget>[];
    if (t.fields.contains('from') || t.fields.contains('province') ||
        t.fields.contains('mine')) {
      final fieldName = t.fields.contains('from')
          ? 'from'
          : t.fields.contains('mine')
              ? 'mine'
              : 'province';
      final label = t.fields.contains('from')
          ? 'From province'
          : t.fields.contains('mine')
              ? 'Province you take'
              : 'Province';
      widgets.add(ProvinceAutocomplete(
        labelText: label,
        suggestions: _relevantProvinces(
          t.label, fieldName, widget.myPower, widget.recipientPower,
          widget.gameState,
        ),
        onSelected: (v) => setState(() {
          _province1 = v;
          // Clear "to" when "from" changes so stale selections don't persist.
          if (t.fields.contains('from') || t.fields.contains('mine')) {
            _province2 = '';
          }
        }),
      ));
      widgets.add(const SizedBox(height: 8));
    }
    if (t.fields.contains('to') || t.fields.contains('yours')) {
      final fieldName = t.fields.contains('to') ? 'to' : 'yours';
      final label = t.fields.contains('to')
          ? 'To province'
          : 'Province they take';
      // Look up province ID from the selected name for cascading.
      final fromId = _provinceIdFromName(_province1);
      widgets.add(ProvinceAutocomplete(
        key: ValueKey('province2_$_province1'),
        labelText: label,
        suggestions: _relevantProvinces(
          t.label, fieldName, widget.myPower, widget.recipientPower,
          widget.gameState,
          fromProvince: fromId,
        ),
        onSelected: (v) => setState(() => _province2 = v),
      ));
      widgets.add(const SizedBox(height: 8));
    }
    if (t.fields.contains('power')) {
      final filteredPowers = allPowers
          .where((p) => p != widget.myPower && p != widget.recipientPower)
          .toList();
      widgets.add(DropdownButtonFormField<String>(
        decoration: const InputDecoration(
          labelText: 'Target power',
          border: OutlineInputBorder(),
          isDense: true,
        ),
        items: filteredPowers
            .map((p) => DropdownMenuItem(value: p, child: Text(p)))
            .toList(),
        onChanged: (v) => setState(() => _power = v ?? ''),
      ));
      widgets.add(const SizedBox(height: 8));
    }
    return widgets;
  }

  bool _canSend() {
    if (_selectedTemplate == null) return false;
    final t = _templates.firstWhere((t) => t.label == _selectedTemplate);
    if ((t.fields.contains('from') || t.fields.contains('province') ||
            t.fields.contains('mine')) &&
        _province1.isEmpty) return false;
    if ((t.fields.contains('to') || t.fields.contains('yours')) &&
        _province2.isEmpty) return false;
    if (t.fields.contains('power') && _power.isEmpty) return false;
    return true;
  }

  void _send() {
    final t = _templates.firstWhere((t) => t.label == _selectedTemplate);
    widget.onSelect(_buildMessage(t));
  }
}

/// Returns land/coastal provinces sorted by relevance for the given template and field.
/// [fromProvince] is the selected "from" province ID, used to cascade the "to" field.
List<Province> _relevantProvinces(
  String templateLabel,
  String fieldName,
  String? myPower,
  String? recipientPower,
  GameState? gs, {
  String? fromProvince,
}) {
  final landCoastal = provinces.values.where((p) => !p.isSea).toList();
  if (gs == null || myPower == null) {
    landCoastal.sort((a, b) => a.name.compareTo(b.name));
    return landCoastal;
  }

  final myUnitLocations = gs.unitsOf(myPower).map((u) => u.province).toSet();
  final myAdjacentSet = <String>{};
  for (final loc in myUnitLocations) {
    myAdjacentSet.addAll(allAdjacent(loc));
  }

  final recipientUnitLocations = recipientPower != null
      ? gs.unitsOf(recipientPower).map((u) => u.province).toSet()
      : <String>{};
  final recipientAdjacentSet = <String>{};
  for (final loc in recipientUnitLocations) {
    recipientAdjacentSet.addAll(allAdjacent(loc));
  }

  final priorityIds = <Set<String>>[];

  switch (templateLabel) {
    case 'Request support':
      if (fieldName == 'from') {
        // Only show my units that the recipient can actually support.
        // A unit can be supported if the recipient has a unit adjacent to
        // the province or adjacent to any of its neighbors.
        if (recipientPower != null && recipientUnitLocations.isNotEmpty) {
          final supportable = myUnitLocations.where((loc) {
            final neighbors = allAdjacent(loc);
            // Recipient has unit adjacent to loc itself.
            if (recipientUnitLocations.intersection(neighbors).isNotEmpty) {
              return true;
            }
            // Recipient has unit adjacent to a neighbor of loc.
            for (final n in neighbors) {
              if (recipientUnitLocations.intersection(allAdjacent(n)).isNotEmpty) {
                return true;
              }
            }
            return false;
          }).toSet();
          priorityIds.add(supportable);
        } else {
          priorityIds.add(myUnitLocations);
        }
      } else {
        // Cascade: if a "from" province is selected, only show its adjacencies.
        if (fromProvince != null && fromProvince.isNotEmpty) {
          final fromAdj = allAdjacent(fromProvince);
          // Further limit to non-sea provinces.
          priorityIds.add(fromAdj.where((id) {
            final p = provinces[id];
            return p != null && !p.isSea;
          }).toSet());
        } else {
          priorityIds.add(myAdjacentSet);
        }
      }
    case 'Non-aggression pact':
      if (recipientPower != null) {
        // Prioritize border provinces between both players' territories.
        final borderProvinces = myAdjacentSet.intersection(
          recipientAdjacentSet.union(recipientUnitLocations),
        ).union(
          recipientAdjacentSet.intersection(myUnitLocations),
        );
        priorityIds.add(borderProvinces);
      } else {
        final mySCs = gs.supplyCenters.entries
            .where((e) => e.value == myPower)
            .map((e) => e.key)
            .toSet();
        final nearSCs = mySCs.intersection(myAdjacentSet)
            ..addAll(mySCs.intersection(myUnitLocations));
        priorityIds.add(supplyCenters.intersection(myAdjacentSet)..addAll(nearSCs));
      }
    case 'Threaten':
      if (recipientPower != null) {
        // Prioritize recipient's SCs adjacent to my units.
        final recipientSCs = gs.supplyCenters.entries
            .where((e) => e.value == recipientPower)
            .map((e) => e.key)
            .toSet();
        final threatenableSCs = recipientSCs.intersection(
          myAdjacentSet.union(myUnitLocations),
        );
        // Also include recipient unit locations adjacent to my units.
        final nearRecipientUnits = recipientUnitLocations.intersection(
          myAdjacentSet.union(myUnitLocations),
        );
        priorityIds.add(threatenableSCs.union(nearRecipientUnits));
      } else {
        final opponentSCs = <String>{};
        priorityIds.add(myAdjacentSet.union(opponentSCs));
      }
    case 'Offer deal':
      if (fieldName == 'mine') {
        final unownedAdj = supplyCenters
            .where((sc) =>
                !gs.supplyCenters.containsKey(sc) ||
                gs.supplyCenters[sc] != myPower)
            .where((sc) => myAdjacentSet.contains(sc) || myUnitLocations.contains(sc))
            .toSet();
        priorityIds.add(unownedAdj);
      } else {
        final unownedAdj = supplyCenters
            .where((sc) =>
                !gs.supplyCenters.containsKey(sc) ||
                gs.supplyCenters[sc] != recipientPower)
            .where((sc) => recipientAdjacentSet.contains(sc) ||
                recipientUnitLocations.contains(sc))
            .toSet();
        priorityIds.add(unownedAdj);
      }
    default:
      break;
  }

  final priority = priorityIds.isEmpty ? <String>{} : priorityIds.first;
  final priorityProvinces = <Province>[];
  final otherProvinces = <Province>[];
  for (final p in landCoastal) {
    if (priority.contains(p.id)) {
      priorityProvinces.add(p);
    } else {
      otherProvinces.add(p);
    }
  }
  priorityProvinces.sort((a, b) => a.name.compareTo(b.name));
  otherProvinces.sort((a, b) => a.name.compareTo(b.name));
  return [...priorityProvinces, ...otherProvinces];
}

class _CannedTemplate {
  final String label;
  final String template;
  final List<String> fields;

  const _CannedTemplate({
    required this.label,
    required this.template,
    required this.fields,
  });
}
