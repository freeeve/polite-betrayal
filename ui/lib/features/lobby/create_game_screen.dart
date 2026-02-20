import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../core/api/api_client.dart';
import '../../core/models/game.dart';
import '../home/game_list_notifier.dart';

const _durations = ['1m', '5m', '10m', '15m', '30m', '1h', '2h', '4h', '8h', '12h', '24h'];
const _difficulties = ['random', 'easy', 'medium', 'hard', 'rust'];

class CreateGameScreen extends ConsumerStatefulWidget {
  const CreateGameScreen({super.key});

  @override
  ConsumerState<CreateGameScreen> createState() => _CreateGameScreenState();
}

class _CreateGameScreenState extends ConsumerState<CreateGameScreen> {
  final _nameController = TextEditingController();
  String _turnDuration = '1m';
  String _retreatDuration = '1m';
  String _buildDuration = '1m';
  String _botDifficulty = 'easy';
  String _powerAssignment = 'random';
  bool _botOnly = false;
  bool _loading = false;
  String? _error;

  Future<void> _create() async {
    final name = _nameController.text.trim();
    if (name.isEmpty) {
      setState(() => _error = 'Game name is required');
      return;
    }

    setState(() {
      _loading = true;
      _error = null;
    });

    final resp = await ref.read(apiClientProvider).post('/games', body: {
      'name': name,
      'turn_duration': _turnDuration,
      'retreat_duration': _retreatDuration,
      'build_duration': _buildDuration,
      'bot_difficulty': _botDifficulty,
      'power_assignment': _powerAssignment,
      'bot_only': _botOnly,
    });

    if (!mounted) return;
    setState(() => _loading = false);

    if (resp.statusCode == 201) {
      final game = Game.fromJson(jsonDecode(resp.body) as Map<String, dynamic>);
      ref.read(openGamesProvider.notifier).refresh();
      ref.read(myGamesProvider.notifier).refresh();
      context.go('/game/${game.id}/lobby');
    } else {
      setState(() => _error = 'Failed to create game: ${resp.body}');
    }
  }

  @override
  void dispose() {
    _nameController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Create Game')),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(24),
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 480),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              TextField(
                controller: _nameController,
                decoration: const InputDecoration(
                  labelText: 'Game Name',
                  border: OutlineInputBorder(),
                ),
              ),
              const SizedBox(height: 24),
              _DurationDropdown(
                label: 'Turn Duration',
                value: _turnDuration,
                onChanged: (v) => setState(() => _turnDuration = v!),
              ),
              const SizedBox(height: 16),
              _DurationDropdown(
                label: 'Retreat Duration',
                value: _retreatDuration,
                onChanged: (v) => setState(() => _retreatDuration = v!),
              ),
              const SizedBox(height: 16),
              _DurationDropdown(
                label: 'Build Duration',
                value: _buildDuration,
                onChanged: (v) => setState(() => _buildDuration = v!),
              ),
              const SizedBox(height: 16),
              DropdownButtonFormField<String>(
                value: _botDifficulty,
                decoration: const InputDecoration(
                  labelText: 'Bot Difficulty',
                  border: OutlineInputBorder(),
                ),
                items: _difficulties
                    .map((d) => DropdownMenuItem(
                          value: d,
                          child: Text(d[0].toUpperCase() + d.substring(1)),
                        ))
                    .toList(),
                onChanged: (v) => setState(() => _botDifficulty = v!),
              ),
              const SizedBox(height: 16),
              DropdownButtonFormField<String>(
                value: _powerAssignment,
                decoration: const InputDecoration(
                  labelText: 'Power Assignment',
                  border: OutlineInputBorder(),
                ),
                items: const [
                  DropdownMenuItem(value: 'random', child: Text('Random')),
                  DropdownMenuItem(value: 'manual', child: Text('Choose in Lobby')),
                ],
                onChanged: (v) => setState(() => _powerAssignment = v!),
              ),
              const SizedBox(height: 16),
              SwitchListTile(
                title: const Text('Bot Only'),
                subtitle: const Text('Watch 7 bots play'),
                value: _botOnly,
                onChanged: (v) => setState(() => _botOnly = v),
                contentPadding: EdgeInsets.zero,
              ),
              const SizedBox(height: 24),
              if (_error != null)
                Padding(
                  padding: const EdgeInsets.only(bottom: 16),
                  child: Text(_error!, style: TextStyle(color: Theme.of(context).colorScheme.error)),
                ),
              FilledButton(
                onPressed: _loading ? null : _create,
                child: _loading
                    ? const SizedBox(height: 20, width: 20, child: CircularProgressIndicator(strokeWidth: 2))
                    : const Text('Create Game'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _DurationDropdown extends StatelessWidget {
  final String label;
  final String value;
  final void Function(String?) onChanged;

  const _DurationDropdown({
    required this.label,
    required this.value,
    required this.onChanged,
  });

  @override
  Widget build(BuildContext context) {
    return DropdownButtonFormField<String>(
      initialValue: value,
      decoration: InputDecoration(
        labelText: label,
        border: const OutlineInputBorder(),
      ),
      items: _durations.map((d) => DropdownMenuItem(value: d, child: Text(d))).toList(),
      onChanged: onChanged,
    );
  }
}
