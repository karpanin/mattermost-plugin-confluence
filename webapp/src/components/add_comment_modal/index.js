import {connect} from 'react-redux';
import {bindActionCreators} from 'redux';
import {getPost} from 'mattermost-redux/selectors/entities/posts';

import {closeAddCommentModal, addCommentToPageFromPost, getCreatePageSpaces, searchCreatePageParents} from '../../actions';
import Selectors from '../../selectors';

import AddCommentModal from './add_comment_modal';

const mapStateToProps = (state) => {
    const modalState = Selectors.getAddCommentModal(state);

    return {
        modalState,
        post: modalState.postId ? getPost(state, modalState.postId) : null,
    };
};

const mapDispatchToProps = (dispatch) => bindActionCreators({
    close: closeAddCommentModal,
    addCommentToPageFromPost,
    getCreatePageSpaces,
    searchCreatePageParents,
}, dispatch);

export default connect(mapStateToProps, mapDispatchToProps)(AddCommentModal);
