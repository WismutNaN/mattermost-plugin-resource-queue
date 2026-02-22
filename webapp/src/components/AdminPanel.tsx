import React, {useState, useEffect} from 'react';
import * as api from '../actions/api';

interface Props {
    theme: any;
    onBack: () => void;
}

const AdminPanel: React.FC<Props> = ({theme, onBack}) => {
    const [resources, setResources] = useState<any[]>([]);
    const [editing, setEditing] = useState<any | null>(null);
    const [form, setForm] = useState({name: '', ip: '', icon: '', description: '', variables: ''});
    const [error, setError] = useState('');
    const [saving, setSaving] = useState(false);

    const load = async () => {
        try {
            const data = await api.getResources();
            setResources(data || []);
        } catch (e: any) {
            setError(e.message);
        }
    };

    useEffect(() => { load(); }, []);

    const resetForm = () => {
        setForm({name: '', ip: '', icon: '', description: '', variables: ''});
        setEditing(null);
    };

    const startEdit = (r: any) => {
        setEditing(r);
        setForm({
            name: r.name || '',
            ip: r.ip || '',
            icon: r.icon || '',
            description: r.description || '',
            variables: r.variables ? Object.entries(r.variables).map(([k, v]) => `${k}=${v}`).join('\n') : '',
        });
    };

    const parseVariables = (text: string): Record<string, string> => {
        const vars: Record<string, string> = {};
        text.split('\n').forEach(line => {
            const eq = line.indexOf('=');
            if (eq > 0) {
                vars[line.substring(0, eq).trim()] = line.substring(eq + 1).trim();
            }
        });
        return vars;
    };

    const save = async () => {
        if (!form.name.trim()) {
            setError('Имя обязательно');
            return;
        }
        setSaving(true);
        setError('');
        try {
            const data = {
                name: form.name.trim(),
                ip: form.ip.trim(),
                icon: form.icon.trim(),
                description: form.description.trim(),
                variables: parseVariables(form.variables),
            };
            if (editing) {
                await api.updateResource(editing.id, data);
            } else {
                await api.createResource(data);
            }
            resetForm();
            await load();
        } catch (e: any) {
            setError(e.message);
        } finally {
            setSaving(false);
        }
    };

    const remove = async (id: string) => {
        if (!confirm('Удалить ресурс? Это удалит все бронирования и очереди.')) return;
        try {
            await api.deleteResource(id);
            await load();
        } catch (e: any) {
            setError(e.message);
        }
    };

    const styles = getStyles(theme);

    return (
        <div>
            {error && <div style={styles.error}>{error}</div>}

            <div style={styles.form}>
                <div style={styles.formTitle}>{editing ? 'Редактировать' : 'Добавить ресурс'}</div>
                <input style={styles.input} placeholder="Имя *" value={form.name}
                    onChange={e => setForm({...form, name: e.target.value})} />
                <input style={styles.input} placeholder="IP адрес" value={form.ip}
                    onChange={e => setForm({...form, ip: e.target.value})} />
                <input style={styles.input} placeholder="Иконка (emoji)" value={form.icon}
                    onChange={e => setForm({...form, icon: e.target.value})} />
                <input style={styles.input} placeholder="Описание" value={form.description}
                    onChange={e => setForm({...form, description: e.target.value})} />
                <textarea style={{...styles.input, minHeight: '50px'}} placeholder="Переменные (key=value, по одной на строку)"
                    value={form.variables} onChange={e => setForm({...form, variables: e.target.value})} />
                <div style={styles.formActions}>
                    <button style={styles.btnPrimary} onClick={save} disabled={saving}>
                        {saving ? '...' : (editing ? 'Сохранить' : 'Добавить')}
                    </button>
                    {editing && <button style={styles.btnSecondary} onClick={resetForm}>Отмена</button>}
                </div>
            </div>

            <div style={styles.list}>
                {resources.map((r: any) => (
                    <div key={r.id} style={styles.listItem}>
                        <div style={styles.listName}>{r.icon || '🖥️'} {r.name}</div>
                        <div style={styles.listMeta}>{r.ip}</div>
                        <div style={styles.listActions}>
                            <button style={styles.btnSmall} onClick={() => startEdit(r)}>✏️</button>
                            <button style={styles.btnSmall} onClick={() => remove(r.id)}>🗑️</button>
                        </div>
                    </div>
                ))}
            </div>
        </div>
    );
};

function getStyles(theme: any) {
    return {
        error: {
            padding: '6px 10px', backgroundColor: '#ffebee', color: '#c62828',
            borderRadius: '4px', marginBottom: '8px', fontSize: '12px',
        },
        form: {
            border: `1px solid ${theme?.centerChannelColor ? theme.centerChannelColor + '22' : '#ddd'}`,
            borderRadius: '6px', padding: '12px', marginBottom: '12px',
        },
        formTitle: {fontSize: '13px', fontWeight: 600 as const, marginBottom: '8px'},
        input: {
            width: '100%', padding: '6px 10px', fontSize: '13px', marginBottom: '6px',
            border: `1px solid ${theme?.centerChannelColor ? theme.centerChannelColor + '33' : '#ccc'}`,
            borderRadius: '4px', boxSizing: 'border-box' as const,
            backgroundColor: theme?.centerChannelBg || '#fff',
            color: theme?.centerChannelColor || '#333',
        },
        formActions: {display: 'flex', gap: '6px', marginTop: '4px'},
        btnPrimary: {
            padding: '5px 14px', fontSize: '12px', border: 'none', borderRadius: '4px',
            cursor: 'pointer', backgroundColor: theme?.buttonBg || '#1976d2',
            color: theme?.buttonColor || '#fff',
        },
        btnSecondary: {
            padding: '5px 14px', fontSize: '12px', borderRadius: '4px', cursor: 'pointer',
            border: `1px solid ${theme?.centerChannelColor ? theme.centerChannelColor + '33' : '#ccc'}`,
            backgroundColor: 'transparent', color: theme?.centerChannelColor || '#333',
        },
        list: {marginTop: '8px'},
        listItem: {
            display: 'flex', alignItems: 'center', gap: '8px',
            padding: '8px 10px', borderRadius: '4px', marginBottom: '4px',
            border: `1px solid ${theme?.centerChannelColor ? theme.centerChannelColor + '15' : '#eee'}`,
        },
        listName: {flex: 1, fontSize: '13px', fontWeight: 500 as const},
        listMeta: {fontSize: '11px', color: theme?.centerChannelColor ? theme.centerChannelColor + '88' : '#999'},
        listActions: {display: 'flex', gap: '4px'},
        btnSmall: {
            background: 'none', border: 'none', cursor: 'pointer', fontSize: '14px', padding: '2px',
        },
    };
}

export default AdminPanel;
