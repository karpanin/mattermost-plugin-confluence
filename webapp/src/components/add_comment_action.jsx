import React from 'react';
import manifest from '../manifest';

export default function AddCommentAction({actionText = 'Add comment to Confluence page'}) {
    return (
        <>
            <span className='MenuItem__icon'>
                <img
                    alt='Confluence'
                    src={`/plugins/${manifest.id}/static/icon.svg`}
                    style={{width: 16, height: 16}}
                />
            </span>
            {actionText}
        </>
    );
}
