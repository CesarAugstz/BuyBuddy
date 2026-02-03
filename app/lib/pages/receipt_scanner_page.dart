import 'package:flutter/material.dart';
import 'package:image_picker/image_picker.dart';
import 'dart:typed_data';
import '../services/receipt_service.dart';
import '../config/theme.dart';

class ReceiptScannerPage extends StatefulWidget {
  const ReceiptScannerPage({super.key});

  @override
  State<ReceiptScannerPage> createState() => _ReceiptScannerPageState();
}

class _ReceiptScannerPageState extends State<ReceiptScannerPage> {
  final _receiptService = ReceiptService();
  final _imagePicker = ImagePicker();

  Uint8List? _imageBytes;
  bool _isProcessing = false;
  Map<String, dynamic>? _extractedData;
  String? _error;

  Future<void> _pickImage(ImageSource source) async {
    try {
      final XFile? image = await _imagePicker.pickImage(
        source: source,
        maxWidth: 1920,
        maxHeight: 1920,
        imageQuality: 85,
      );

      if (image != null) {
        final bytes = await image.readAsBytes();
        setState(() {
          _imageBytes = bytes;
          _extractedData = null;
          _error = null;
        });
        await _processImage();
      }
    } catch (e) {
      setState(() {
        _error = 'Failed to pick image: $e';
      });
    }
  }

  Future<void> _processImage() async {
    if (_imageBytes == null) return;

    setState(() {
      _isProcessing = true;
      _error = null;
    });

    try {
      final result = await _receiptService.processReceipt(_imageBytes!);

      if (result['success'] == true) {
        setState(() {
          _extractedData = result['data'];
          _isProcessing = false;
        });
      } else {
        setState(() {
          _error = result['error'] ?? 'Failed to process receipt';
          _isProcessing = false;
        });
      }
    } catch (e) {
      setState(() {
        _error = 'Error processing receipt: $e';
        _isProcessing = false;
      });
    }
  }

  Future<void> _confirmAndSave() async {
    if (_extractedData == null) return;

    setState(() => _isProcessing = true);

    try {
      final saved = await _receiptService.saveReceipt(_extractedData!);

      if (!saved) {
        throw Exception('Failed to save receipt');
      }

      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Receipt saved successfully!'),
            backgroundColor: Colors.green,
          ),
        );
        Navigator.pop(context);
      }
    } catch (e) {
      setState(() {
        _error = 'Error saving receipt: $e';
        _isProcessing = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.white,
      appBar: AppBar(title: const Text('Scan Receipt')),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(24),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            if (_imageBytes == null) ...[
              Container(
                height: 300,
                decoration: BoxDecoration(
                  color: AppTheme.lightGray,
                  borderRadius: BorderRadius.circular(12),
                  border: Border.all(color: AppTheme.darkGray, width: 2),
                ),
                child: Center(
                  child: Column(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      Icon(
                        Icons.receipt_long,
                        size: 64,
                        color: AppTheme.darkGray,
                      ),
                      const SizedBox(height: 16),
                      Text(
                        'No image selected',
                        style: TextStyle(
                          fontSize: 16,
                          color: AppTheme.darkGray,
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ] else ...[
              ClipRRect(
                borderRadius: BorderRadius.circular(12),
                child: Image.memory(
                  _imageBytes!,
                  height: 300,
                  fit: BoxFit.cover,
                ),
              ),
            ],
            const SizedBox(height: 24),
            Row(
              children: [
                Expanded(
                  child: ElevatedButton.icon(
                    onPressed:
                        _isProcessing
                            ? null
                            : () => _pickImage(ImageSource.camera),
                    icon: const Icon(Icons.camera_alt),
                    label: const Text('Camera'),
                    style: ElevatedButton.styleFrom(
                      padding: const EdgeInsets.symmetric(vertical: 16),
                    ),
                  ),
                ),
                const SizedBox(width: 16),
                Expanded(
                  child: ElevatedButton.icon(
                    onPressed:
                        _isProcessing
                            ? null
                            : () => _pickImage(ImageSource.gallery),
                    icon: const Icon(Icons.photo_library),
                    label: const Text('Gallery'),
                    style: ElevatedButton.styleFrom(
                      padding: const EdgeInsets.symmetric(vertical: 16),
                    ),
                  ),
                ),
              ],
            ),
            if (_isProcessing) ...[
              const SizedBox(height: 32),
              const Center(child: CircularProgressIndicator()),
              const SizedBox(height: 16),
              const Center(child: Text('Processing receipt...')),
            ],
            if (_error != null) ...[
              const SizedBox(height: 24),
              Container(
                padding: const EdgeInsets.all(16),
                decoration: BoxDecoration(
                  color: Colors.red.shade50,
                  borderRadius: BorderRadius.circular(8),
                  border: Border.all(color: Colors.red.shade300),
                ),
                child: Row(
                  children: [
                    Icon(Icons.error_outline, color: Colors.red.shade700),
                    const SizedBox(width: 12),
                    Expanded(
                      child: Text(
                        _error!,
                        style: TextStyle(color: Colors.red.shade700),
                      ),
                    ),
                  ],
                ),
              ),
            ],
            if (_extractedData != null) ...[
              const SizedBox(height: 32),
              Text(
                'Extracted Information',
                style: TextStyle(
                  fontSize: 20,
                  fontWeight: FontWeight.w600,
                  color: AppTheme.nearBlack,
                ),
              ),
              const SizedBox(height: 16),
              _buildDataCard(
                'Company',
                _extractedData!['company'] ?? 'Not found',
              ),
              const SizedBox(height: 12),
              _buildDataCard(
                'Total',
                'R\$ ${_extractedData!['total'] ?? '0.00'}',
              ),
              if (_extractedData!['accessKey'] != null &&
                  _extractedData!['accessKey'] != '') ...[
                const SizedBox(height: 12),
                _buildDataCard(
                  'Access Key',
                  _extractedData!['accessKey'],
                  isMonospace: true,
                ),
              ],
              const SizedBox(height: 16),
              Text(
                'Items',
                style: TextStyle(
                  fontSize: 16,
                  fontWeight: FontWeight.w600,
                  color: AppTheme.nearBlack,
                ),
              ),
              const SizedBox(height: 8),
              if (_extractedData!['items'] != null)
                ...(_extractedData!['items'] as List).map(
                  (item) => _buildItemCard(item),
                ),
              const SizedBox(height: 24),
              ElevatedButton(
                onPressed: _isProcessing ? null : _confirmAndSave,
                style: ElevatedButton.styleFrom(
                  padding: const EdgeInsets.symmetric(vertical: 16),
                  backgroundColor: Colors.green,
                ),
                child: const Text(
                  'Confirm & Save',
                  style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildDataCard(
    String label,
    String value, {
    bool isMonospace = false,
  }) {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: AppTheme.lightGray,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            label,
            style: TextStyle(
              fontWeight: FontWeight.w600,
              color: AppTheme.darkGray,
              fontSize: 12,
            ),
          ),
          const SizedBox(height: 4),
          Text(
            value,
            style: TextStyle(
              fontWeight: FontWeight.w600,
              fontFamily: isMonospace ? 'monospace' : null,
              fontSize: isMonospace ? 11 : 16,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildItemCard(Map<String, dynamic> item) {
    final brand = item['brand'] as String?;
    final quantity = item['quantity'];
    final unit = item['unit'] as String?;
    final unitPrice = item['unitPrice'];
    final totalPrice = item['totalPrice'];
    final category = item['category'] as String?;
    final subcategory = item['subcategory'] as String?;

    return Container(
      margin: const EdgeInsets.only(bottom: 12),
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: AppTheme.lightGray),
        boxShadow: [
          BoxShadow(
            color: Colors.black.withOpacity(0.05),
            blurRadius: 4,
            offset: const Offset(0, 2),
          ),
        ],
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      item['name'] ?? 'Unknown item',
                      style: const TextStyle(
                        fontWeight: FontWeight.w600,
                        fontSize: 16,
                      ),
                    ),
                    if (brand != null) ...[
                      const SizedBox(height: 4),
                      Text(
                        brand,
                        style: TextStyle(
                          fontSize: 13,
                          color: AppTheme.darkGray,
                          fontStyle: FontStyle.italic,
                        ),
                      ),
                    ],
                  ],
                ),
              ),
              const SizedBox(width: 12),
              Text(
                'R\$ ${totalPrice?.toStringAsFixed(2) ?? '0.00'}',
                style: const TextStyle(
                  fontWeight: FontWeight.w700,
                  fontSize: 16,
                  color: Colors.green,
                ),
              ),
            ],
          ),
          const SizedBox(height: 12),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              if (quantity != null && unit != null)
                _buildInfoChip(
                  Icons.shopping_basket,
                  '$quantity $unit',
                  Colors.blue,
                ),
              if (unitPrice != null)
                _buildInfoChip(
                  null,
                  'R\$ ${unitPrice.toStringAsFixed(2)}/$unit',
                  Colors.orange,
                ),
              if (category != null)
                _buildInfoChip(Icons.category, category, Colors.purple),
              if (subcategory != null)
                _buildInfoChip(Icons.label, subcategory, Colors.teal),
            ],
          ),
        ],
      ),
    );
  }

  Widget _buildInfoChip(IconData? icon, String label, Color color) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
      decoration: BoxDecoration(
        color: color.withAlpha(25),
        borderRadius: BorderRadius.circular(20),
        border: Border.all(color: color.withAlpha(25)),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 14, color: color),
          const SizedBox(width: 4),
          Text(
            label,
            style: TextStyle(
              fontSize: 12,
              fontWeight: FontWeight.w500,
              color: color,
            ),
          ),
        ],
      ),
    );
  }
}
