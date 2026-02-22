import React, {useState, useEffect} from 'react';
import * as api from '../actions/api';

interface Props {
    resourceId: string;
    mode: 'book' | 'queue' | 'extend';
    theme: any;
    onClose: () => void;
    onDone: () => void;
}

const TITLES: Record<string, string> = {
    book: '🔒 Забронировать',
    queue: '📋 Встать в очередь',
    extend: '⏳ Продлить',
};

const BookingModal: React.FC<Props> = ({resourceId, mode, theme, onClose, onDone}) => {
    const [presets, setPresets] = useState<any[]>([]);
    const [customMinutes, setCustomMinutes] = useState('');
    const [purpose, setPurpose] = useState('');
    const [error, setError] = useState('');
    const [submitting, setSubmitting] = useState(false);

    useEffect(() => {
        api.getPresets().then(setPresets).catch(() => {});
    }, []);

    const submit = async (minutes: number) => {
        if (minutes <= 0) { setError('Укажите время'); return; }
        setSubmitting(true);
        setError('');
        try {
            if (mode === 'book') {
                await api.bookResource(resourceId, minutes, purpose);
            } else if (mode === 'queue') {
                await api.joinQueue(resourceId, minutes, purpose);
            } else if (mode === 'extend') {
                await api.extendResource(resourceId, minutes);
            }
            onDone();
        } catch (e: any) {
            setError(e.message);
        } finally {
            setSubmitting(false);
        }
    };

    const styles = getStyles(theme);

    return (
        <div style={styles.overlay} onClick={onClose}>
            <div style={styles.modal} onClick={(e) => e.stopPropagation()}>
                <div style={styles.modalHeader}>{TITLES[mode] || mode}</div>

                {error && <div style={styles.error}>{error}</div>}

                <div style={styles.section}>
                    <label style={styles.label}>Быстрый выбор:</label>
                    <div style={styles.presets}>
                        {presets.map((p) => (
                            <button key={p.minutes} style={styles.presetBtn}
                                onClick={() => submit(p.minutes)} disabled={submitting}>
                                {p.label}
                            </button>
                        ))}
                    </div>
                </div>

                <div style={styles.section}>
                    <label style={styles.label}>Своё время (минуты):</label>
                    <div style={styles.customRow}>
                        <input type="number" style={styles.input} value={customMinutes}
                            onChange={(e) => setCustomMinutes(e.target.value)}
                            placeholder="60" min="1"
                            onKeyDown={(e) => e.key === 'Enter' && submit(parseInt(customMinutes, 10) || 0)}
                        />
                        <button style={styles.submitBtn}
                            onClick={() => submit(parseInt(customMinutes, 10) || 0)}
                            disabled={submitting}>
                            {submitting ? '...' : 'OK'}
                        </button>
                    </div>
                </div>

                {mode !== 'extend' && (
                    <div style={styles.section}>
                        <label style={styles.label}>Цель (необязательно):</label>
                        <input type="text" style={styles.input} value={purpose}
                            onChange={(e) => setPurpose(e.target.value)}
                            placeholder="Моделирование, тестирование..."
                        />
                    </div>
                )}

                <button style={styles.cancelBtn} onClick={onClose}>Отмена</button>
            </div>
        </div>
    );
};

function getStyles(theme: any) {
    const fgFaint = (theme?.centerChannelColor || '#333') + '33';
    return {
        overlay: {
            position: 'fixed' as const, top: 0, left: 0, right: 0, bottom: 0,
            backgroundColor: 'rgba(0,0,0,0.4)',
            display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 10000,
        },
        modal: {
            backgroundColor: theme?.centerChannelBg || '#fff',
            color: theme?.centerChannelColor || '#333',
            borderRadius: '8px', padding: '20px', width: '320px',
            maxHeight: '80vh', overflowY: 'auto' as const,
            boxShadow: '0 4px 20px rgba(0,0,0,0.2)',
        },
        modalHeader: { fontSize: '16px', fontWeight: 600 as const, marginBottom: '16px' },
        section: { marginBottom: '14px' },
        label: {
            fontSize: '12px', fontWeight: 500 as const, marginBottom: '6px', display: 'block',
            color: (theme?.centerChannelColor || '#555') + 'cc',
        },
        presets: { display: 'flex', flexWrap: 'wrap' as const, gap: '6px' },
        presetBtn: {
            padding: '6px 12px', fontSize: '12px',
            border: `1px solid ${theme?.buttonBg || '#1976d2'}`, borderRadius: '4px',
            cursor: 'pointer', backgroundColor: 'transparent',
            color: theme?.buttonBg || '#1976d2',
        },
        customRow: { display: 'flex', gap: '6px' },
        input: {
            flex: 1, padding: '6px 10px', fontSize: '13px',
            border: `1px solid ${fgFaint}`, borderRadius: '4px',
            backgroundColor: theme?.centerChannelBg || '#fff',
            color: theme?.centerChannelColor || '#333', outline: 'none',
            boxSizing: 'border-box' as const, width: '100%',
        },
        submitBtn: {
            padding: '6px 16px', fontSize: '12px', border: 'none', borderRadius: '4px',
            cursor: 'pointer', backgroundColor: theme?.buttonBg || '#1976d2',
            color: theme?.buttonColor || '#fff',
        },
        cancelBtn: {
            padding: '6px 16px', fontSize: '12px', borderRadius: '4px',
            cursor: 'pointer', backgroundColor: 'transparent',
            border: `1px solid ${fgFaint}`, color: theme?.centerChannelColor || '#333',
            width: '100%',
        },
        error: {
            padding: '6px 10px', backgroundColor: '#ffebee', color: '#c62828',
            borderRadius: '4px', marginBottom: '12px', fontSize: '12px',
        },
    };
}

export default BookingModal;
