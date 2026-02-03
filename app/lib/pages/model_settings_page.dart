import 'package:flutter/material.dart';
import 'package:buybuddy/services/preferences_service.dart';

class ModelSettingsPage extends StatefulWidget {
  const ModelSettingsPage({super.key});

  @override
  State<ModelSettingsPage> createState() => _ModelSettingsPageState();
}

class _ModelSettingsPageState extends State<ModelSettingsPage> {
  final PreferencesService _prefsService = PreferencesService();
  
  bool _isLoading = true;
  bool _isSaving = false;
  
  UserPreferences? _currentPrefs;
  AvailableModels? _availableModels;
  
  String? _selectedReceiptModel;
  String? _selectedAssistantModel;

  @override
  void initState() {
    super.initState();
    _loadData();
  }

  Future<void> _loadData() async {
    setState(() => _isLoading = true);
    
    try {
      final prefs = await _prefsService.getPreferences();
      final models = await _prefsService.getAvailableModels();
      
      setState(() {
        _currentPrefs = prefs;
        _availableModels = models;
        _selectedReceiptModel = prefs.receiptModel;
        _selectedAssistantModel = prefs.assistantModel;
        _isLoading = false;
      });
    } catch (e) {
      setState(() => _isLoading = false);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Error loading settings: $e')),
        );
      }
    }
  }

  Future<void> _savePreferences() async {
    if (_selectedReceiptModel == null || _selectedAssistantModel == null) {
      return;
    }

    setState(() => _isSaving = true);

    try {
      final newPrefs = UserPreferences(
        receiptModel: _selectedReceiptModel!,
        assistantModel: _selectedAssistantModel!,
      );
      
      await _prefsService.updatePreferences(newPrefs);
      
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Settings saved successfully!'),
            backgroundColor: Colors.green,
          ),
        );
        Navigator.pop(context);
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Error saving settings: $e')),
        );
      }
    } finally {
      setState(() => _isSaving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('AI Model Settings'),
        actions: [
          if (!_isLoading && !_isSaving)
            IconButton(
              icon: const Icon(Icons.save),
              onPressed: _savePreferences,
            ),
          if (_isSaving)
            const Padding(
              padding: EdgeInsets.all(16.0),
              child: SizedBox(
                width: 24,
                height: 24,
                child: CircularProgressIndicator(strokeWidth: 2),
              ),
            ),
        ],
      ),
      body: _isLoading
          ? const Center(child: CircularProgressIndicator())
          : SingleChildScrollView(
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'Receipt Processing Model',
                    style: Theme.of(context).textTheme.titleLarge,
                  ),
                  const SizedBox(height: 8),
                  Text(
                    'Choose which AI model to use for scanning receipts',
                    style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                          color: Colors.grey[600],
                        ),
                  ),
                  const SizedBox(height: 16),
                  ..._buildModelOptions(
                    _availableModels?.receiptModels ?? [],
                    _selectedReceiptModel,
                    (value) => setState(() => _selectedReceiptModel = value),
                  ),
                  const SizedBox(height: 32),
                  Text(
                    'Shopping Assistant Model',
                    style: Theme.of(context).textTheme.titleLarge,
                  ),
                  const SizedBox(height: 8),
                  Text(
                    'Choose which AI model to use for the shopping assistant',
                    style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                          color: Colors.grey[600],
                        ),
                  ),
                  const SizedBox(height: 16),
                  ..._buildModelOptions(
                    _availableModels?.assistantModels ?? [],
                    _selectedAssistantModel,
                    (value) => setState(() => _selectedAssistantModel = value),
                  ),
                  const SizedBox(height: 32),
                  SizedBox(
                    width: double.infinity,
                    child: ElevatedButton(
                      onPressed: _isSaving ? null : _savePreferences,
                      style: ElevatedButton.styleFrom(
                        padding: const EdgeInsets.symmetric(vertical: 16),
                      ),
                      child: _isSaving
                          ? const SizedBox(
                              width: 20,
                              height: 20,
                              child: CircularProgressIndicator(strokeWidth: 2),
                            )
                          : const Text('Save Settings'),
                    ),
                  ),
                ],
              ),
            ),
    );
  }

  List<Widget> _buildModelOptions(
    List<GeminiModel> models,
    String? selectedValue,
    ValueChanged<String?> onChanged,
  ) {
    return models.map((model) {
      return Card(
        margin: const EdgeInsets.only(bottom: 8),
        child: RadioListTile<String>(
          value: model.id,
          groupValue: selectedValue,
          onChanged: onChanged,
          title: Text(model.name),
          subtitle: Text(model.description),
          activeColor: Theme.of(context).primaryColor,
        ),
      );
    }).toList();
  }
}
