import React, {useState} from 'react';

interface Props {
    status: any;
    theme: any;
    isAdmin: boolean;
    onBook: () => void;
    onQueue: () => void;
    onRelease: () => void;
    onExtend: () => void;
    onLeaveQueue: () => void;
    onSubscribe: () => void;
    onUnsubscribe: () => void;
    onHistory: () => void;
}

const ResourceCard: React.FC<Props> = ({
    status, theme, isAdmin,
    onBook, onQueue, onRelease, onExtend, onLeaveQueue,
    onSubscribe, onUnsubscribe, onHistory,
}) => {
    const [expanded, setExpanded] = useState(false);
    const {resource, booking, queue, subscribers, is_holder, in_queue, is_subscribed} = status;
    const isBooked = !!booking;
    const icon = resource.icon || '🖥️';

    const timeLeft = isBooked ? Math.max(0, Math.floor((new Date(booking.expires_at).getTime() - Date.now()) / 1000)) : 0;
    const timeLeftStr = formatSeconds(timeLeft);

    const styles = getStyles(theme, isBooked);

    return (
        <div style={styles.card}>
            <div style={styles.cardHeader} onClick={() => setExpanded(!expanded)}>
                <div style={styles.nameRow}>
                    <span style={styles.icon}>{icon}</span>
                    <span style={styles.name}>{resource.name}</span>
                    <span style={styles.statusDot}>{isBooked ? '🔴' : '🟢'}</span>
                    <span style={styles.expandArrow}>{expanded ? '▾' : '▸'}</span>
                </div>
                {isBooked && (
                    <div style={styles.bookingInfo}>
                        <span style={styles.username}>
                            {is_holder ? '📌 Вы' : `@${booking.username}`}
                        </span>
                        <span style={styles.timeLeft}>⏱ {timeLeftStr}</span>
                    </div>
                )}
                {!isBooked && (
                    <div style={styles.freeLabel}>Свободен</div>
                )}
                {queue && queue.length > 0 && (
                    <span style={styles.queueBadge}>
                        👥 Очередь: {queue.length}{in_queue ? ' (вы в ней)' : ''}
                    </span>
                )}
            </div>

            {expanded && (
                <div style={styles.details}>
                    {resource.ip && <div style={styles.detailRow}>📡 <code>{resource.ip}</code></div>}
                    {resource.description && <div style={styles.detailRow}>{resource.description}</div>}
                    {resource.variables && Object.keys(resource.variables).length > 0 && (
                        <div style={styles.varsBlock}>
                            {Object.entries(resource.variables).map(([k, v]) => (
                                <div key={k} style={styles.varRow}>
                                    <span style={styles.varKey}>{k}</span>
                                    <span style={styles.varVal}>{v as string}</span>
                                </div>
                            ))}
                        </div>
                    )}
                    {isBooked && booking.purpose && (
                        <div style={styles.detailRow}>🎯 {booking.purpose}</div>
                    )}

                    {queue && queue.length > 0 && (
                        <div style={styles.queueList}>
                            <div style={styles.subTitle}>Очередь:</div>
                            {queue.map((e: any, i: number) => (
                                <div key={i} style={styles.queueEntry}>
                                    {i + 1}. @{e.username}
                                    {e.purpose && <span style={styles.queuePurpose}> — {e.purpose}</span>}
                                </div>
                            ))}
                        </div>
                    )}

                    <div style={styles.actions}>
                        {/* Free resource — anyone can book */}
                        {!isBooked && (
                            <button style={styles.btnPrimary} onClick={onBook}>🔒 Занять</button>
                        )}

                        {/* I hold the resource */}
                        {isBooked && is_holder && (
                            <>
                                <button style={styles.btnDanger} onClick={onRelease}>🔓 Освободить</button>
                                <button style={styles.btnAccent} onClick={onExtend}>⏳ Продлить</button>
                            </>
                        )}

                        {/* Someone else holds it — I can queue */}
                        {isBooked && !is_holder && !in_queue && (
                            <button style={styles.btnPrimary} onClick={onQueue}>📋 В очередь</button>
                        )}

                        {/* I'm in queue — I can leave */}
                        {in_queue && (
                            <button style={styles.btnSecondary} onClick={onLeaveQueue}>🚪 Покинуть очередь</button>
                        )}

                        {/* Admin can force release */}
                        {isBooked && !is_holder && isAdmin && (
                            <button style={styles.btnDanger} onClick={onRelease}>⚡ Освободить (админ)</button>
                        )}

                        {/* Subscribe / unsubscribe */}
                        {is_subscribed ? (
                            <button style={styles.btnSub} onClick={onUnsubscribe} title="Отписаться от уведомлений">🔕</button>
                        ) : (
                            <button style={styles.btnSub} onClick={onSubscribe} title="Подписаться на уведомления">🔔</button>
                        )}

                        <button style={styles.btnSub} onClick={onHistory} title="История">📊</button>
                    </div>

                    <div style={styles.meta}>
                        {is_subscribed && <span style={styles.metaTag}>✓ подписка</span>}
                        <span style={styles.metaTag}>👁 {subscribers}</span>
                    </div>
                </div>
            )}
        </div>
    );
};

function formatSeconds(s: number): string {
    if (s <= 0) return 'истекает...';
    const h = Math.floor(s / 3600);
    const m = Math.floor((s % 3600) / 60);
    if (h > 0) return `${h}ч ${m}м`;
    if (m > 0) return `${m}м`;
    return `${s}с`;
}

function getStyles(theme: any, isBooked: boolean) {
    const border = isBooked ? (theme?.errorTextColor || '#e53935') : (theme?.onlineIndicator || '#4caf50');
    const bg = theme?.centerChannelBg || '#fff';
    const fg = theme?.centerChannelColor || '#333';
    const fgSoft = fg + (fg.length <= 4 ? '8' : '88');
    const fgFaint = fg + (fg.length <= 4 ? '3' : '22');

    return {
        card: {
            border: `1px solid ${fgFaint}`,
            borderLeft: `3px solid ${border}`,
            borderRadius: '6px',
            marginBottom: '8px',
            overflow: 'hidden' as const,
            backgroundColor: bg,
        },
        cardHeader: { padding: '10px 12px', cursor: 'pointer', userSelect: 'none' as const },
        nameRow: { display: 'flex', alignItems: 'center', gap: '6px' },
        icon: { fontSize: '15px' },
        name: { fontWeight: 600 as const, fontSize: '14px', flex: 1 },
        statusDot: { fontSize: '10px' },
        expandArrow: { fontSize: '11px', color: fgSoft },
        bookingInfo: {
            display: 'flex', justifyContent: 'space-between', marginTop: '4px',
            fontSize: '12px', color: fgSoft,
        },
        freeLabel: { fontSize: '12px', color: theme?.onlineIndicator || '#4caf50', marginTop: '2px' },
        username: {},
        timeLeft: { fontWeight: 500 as const, color: isBooked && theme?.errorTextColor ? theme.errorTextColor : fgSoft },
        queueBadge: { fontSize: '11px', color: theme?.linkColor || '#1976d2', marginTop: '2px', display: 'block' },
        details: {
            padding: '0 12px 10px', borderTop: `1px solid ${fgFaint}`, fontSize: '13px',
        },
        detailRow: { marginTop: '6px', lineHeight: 1.4 },
        varsBlock: {
            marginTop: '6px', padding: '6px 8px',
            backgroundColor: fgFaint, borderRadius: '4px',
        },
        varRow: { display: 'flex', justifyContent: 'space-between', fontSize: '11px', padding: '1px 0' },
        varKey: { fontWeight: 500 as const, fontFamily: 'monospace' },
        varVal: { fontFamily: 'monospace', color: fgSoft },
        subTitle: { fontSize: '12px', fontWeight: 600 as const, marginTop: '8px', marginBottom: '4px' },
        queueList: { marginTop: '4px' },
        queueEntry: { fontSize: '12px', marginLeft: '8px', padding: '2px 0', color: fgSoft },
        queuePurpose: { fontStyle: 'italic' as const },
        actions: { display: 'flex', gap: '6px', marginTop: '10px', flexWrap: 'wrap' as const },
        btnPrimary: {
            padding: '5px 12px', fontSize: '12px', fontWeight: 500 as const,
            border: 'none', borderRadius: '4px', cursor: 'pointer',
            backgroundColor: theme?.buttonBg || '#1976d2', color: theme?.buttonColor || '#fff',
        },
        btnDanger: {
            padding: '5px 12px', fontSize: '12px', fontWeight: 500 as const,
            border: 'none', borderRadius: '4px', cursor: 'pointer',
            backgroundColor: theme?.errorTextColor || '#e53935', color: '#fff',
        },
        btnAccent: {
            padding: '5px 12px', fontSize: '12px', fontWeight: 500 as const,
            border: 'none', borderRadius: '4px', cursor: 'pointer',
            backgroundColor: theme?.mentionHighlightBg || '#ff9800', color: '#fff',
        },
        btnSecondary: {
            padding: '5px 12px', fontSize: '12px',
            border: `1px solid ${fgFaint}`, borderRadius: '4px', cursor: 'pointer',
            backgroundColor: 'transparent', color: fg,
        },
        btnSub: {
            padding: '5px 8px', fontSize: '14px',
            border: `1px solid ${fgFaint}`, borderRadius: '4px', cursor: 'pointer',
            backgroundColor: 'transparent', lineHeight: 1,
        },
        meta: {
            display: 'flex', gap: '8px', marginTop: '8px', fontSize: '11px', color: fgSoft,
        },
        metaTag: {},
    };
}

export default ResourceCard;
