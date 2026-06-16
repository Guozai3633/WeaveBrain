import 'package:flutter/material.dart';

import '../../../shared/models/project.dart';

class ProjectFormDialog extends StatefulWidget {
  final Project? project;

  const ProjectFormDialog({super.key, this.project});

  @override
  State<ProjectFormDialog> createState() => _ProjectFormDialogState();
}

class _ProjectFormDialogState extends State<ProjectFormDialog> {
  late final TextEditingController _nameController;
  bool _defaultProject = false;

  bool get _isEditing => widget.project != null;

  @override
  void initState() {
    super.initState();
    _nameController = TextEditingController(text: widget.project?.name ?? '');
    _defaultProject = widget.project?.defaultProject ?? false;
  }

  @override
  void dispose() {
    _nameController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: Text(_isEditing ? '编辑项目' : '创建项目'),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          TextField(
            controller: _nameController,
            decoration: const InputDecoration(
              labelText: '项目名称',
              hintText: '输入项目名称',
            ),
            autofocus: true,
          ),
          const SizedBox(height: 16),
          CheckboxListTile(
            value: _defaultProject,
            onChanged: (v) => setState(() => _defaultProject = v ?? false),
            title: const Text('设为默认项目'),
            controlAffinity: ListTileControlAffinity.leading,
            contentPadding: EdgeInsets.zero,
          ),
        ],
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(context),
          child: const Text('取消'),
        ),
        FilledButton(
          onPressed: _nameController.text.trim().isEmpty
              ? null
              : () {
                  Navigator.pop(
                    context,
                    (
                      name: _nameController.text.trim(),
                      defaultProject: _defaultProject,
                    ),
                  );
                },
          child: Text(_isEditing ? '保存' : '创建'),
        ),
      ],
    );
  }
}
