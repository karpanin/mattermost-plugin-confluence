import React from 'react';
import PropTypes from 'prop-types';
import {
    Button,
    Modal,
} from 'react-bootstrap';

import ConfluenceField from '../confluence_field';
import Validator from '../validator';

const initialState = {
    selectedSpace: null,
    selectedPage: null,
    pageQuery: '',
    spaces: [],
    pageOptions: [],
    error: '',
    saving: false,
    loadingSpaces: false,
    loadingPages: false,
};

const normalizePageOptions = (options) => {
    if (!Array.isArray(options)) {
        return [];
    }

    return options.map((option) => ({
        ...option,
        value: String(option?.value ?? ''),
        label: String(option?.label ?? option?.value ?? ''),
    }));
};

export default class AddCommentModal extends React.PureComponent {
    static propTypes = {
        modalState: PropTypes.object.isRequired,
        post: PropTypes.object,
        theme: PropTypes.object,
        close: PropTypes.func.isRequired,
        addCommentToPageFromPost: PropTypes.func.isRequired,
        getCreatePageSpaces: PropTypes.func.isRequired,
        searchCreatePageParents: PropTypes.func.isRequired,
    };

    static defaultProps = {
        post: null,
        theme: null,
    };

    constructor(props) {
        super(props);
        this.state = initialState;
        this.validator = new Validator();
        this.pageSearchRequest = 0;
    }

    componentDidUpdate(prevProps) {
        if (this.props.modalState.postId && this.props.modalState.postId !== prevProps.modalState.postId) {
            this.setState(initialState, this.loadSpaces);
        }
    }

    loadSpaces = async () => {
        this.setState({loadingSpaces: true});
        const response = await this.props.getCreatePageSpaces();
        const payload = response.data || {};
        const spaces = Array.isArray(payload.spaces) ? payload.spaces : [];
        const lastSelectedSpaceKey = payload.lastSelectedSpaceKey;
        const selectedSpace = spaces.find((space) => space.value === lastSelectedSpaceKey) || null;
        this.setState({
            loadingSpaces: false,
            spaces,
            selectedSpace,
            error: response.error ? (response.error.response?.text || 'Failed to load Confluence spaces.') : '',
        });
    };

    handleClose = (e) => {
        if (e && e.preventDefault) {
            e.preventDefault();
        }

        this.pageSearchRequest += 1;
        this.setState(initialState, this.props.close);
    };

    handleSpaceChange = (selectedSpace) => {
        this.pageSearchRequest += 1;
        this.setState({
            selectedSpace,
            selectedPage: null,
            pageOptions: [],
            pageQuery: '',
            error: '',
        });
    };

    handlePageChange = (selectedPage) => {
        this.setState({selectedPage});
    };

    handlePageSearch = (query, meta) => {
        if (meta?.action && meta.action !== 'input-change') {
            return query;
        }

        this.setState({pageQuery: query, error: ''});
        if (!this.state.selectedSpace || !query || query.trim().length < 2) {
            this.setState({
                pageOptions: [],
                loadingPages: false,
            });
            return query;
        }

        const requestID = this.pageSearchRequest + 1;
        this.pageSearchRequest = requestID;
        this.setState({loadingPages: true});
        this.searchPages(requestID, this.state.selectedSpace.value, query.trim());
        return query;
    };

    searchPages = async (requestID, spaceKey, query) => {
        const response = await this.props.searchCreatePageParents(spaceKey, query);
        if (requestID !== this.pageSearchRequest) {
            return;
        }

        this.setState({
            loadingPages: false,
            pageOptions: normalizePageOptions(response.data),
            error: response.error ? (response.error.response?.text || 'Failed to search Confluence pages.') : '',
        });
    };

    handleSubmit = async () => {
        if (!this.validator.validate()) {
            return;
        }

        this.setState({
            saving: true,
            error: '',
        });

        const response = await this.props.addCommentToPageFromPost({
            postID: this.props.modalState.postId,
            pageID: this.state.selectedPage?.value,
            spaceKey: this.state.selectedSpace?.value,
        });

        if (response.error) {
            this.setState({
                saving: false,
                error: response.error.response?.text || 'Failed to add Confluence comment.',
            });
            return;
        }

        this.handleClose();
    };

    render() {
        const visible = Boolean(this.props.modalState.postId);
        const {saving, error, loadingSpaces, loadingPages} = this.state;
        const theme = this.props.theme || {};
        const modalStyles = {
            header: {
                color: theme.centerChannelColor,
                backgroundColor: theme.centerChannelBg,
            },
            body: {
                color: theme.centerChannelColor,
                backgroundColor: theme.centerChannelBg,
            },
            footer: {
                color: theme.centerChannelColor,
                backgroundColor: theme.centerChannelBg,
            },
        };

        return (
            <Modal
                show={visible}
                onHide={this.handleClose}
                backdrop={'static'}
            >
                <Modal.Header closeButton={true} style={modalStyles.header}>
                    <Modal.Title>{'Add Comment to Confluence Page'}</Modal.Title>
                </Modal.Header>
                <Modal.Body style={modalStyles.body}>
                    <ConfluenceField
                        label={'Space'}
                        fieldType={'dropDown'}
                        required={true}
                        placeholder={'Select a Confluence space.'}
                        value={this.state.selectedSpace}
                        options={this.state.spaces}
                        theme={theme}
                        isSearchable={true}
                        isMulti={false}
                        addValidation={this.validator.addValidation}
                        removeValidation={this.validator.removeValidation}
                        onChange={this.handleSpaceChange}
                        isLoading={loadingSpaces}
                        noOptionsMessage={() => loadingSpaces ? 'Loading spaces...' : 'No available spaces found.'}
                    />
                    <ConfluenceField
                        label={'Page'}
                        fieldType={'dropDown'}
                        required={true}
                        placeholder={this.state.selectedSpace ? 'Search page by title.' : 'Select a space first.'}
                        value={this.state.selectedPage}
                        options={this.state.pageOptions}
                        theme={theme}
                        isSearchable={true}
                        isMulti={false}
                        addValidation={this.validator.addValidation}
                        removeValidation={this.validator.removeValidation}
                        onChange={this.handlePageChange}
                        onInputChange={this.handlePageSearch}
                        filterOption={() => true}
                        isDisabled={!this.state.selectedSpace}
                        isLoading={loadingPages}
                        noOptionsMessage={() => {
                            if (!this.state.selectedSpace) {
                                return 'Select a space first.';
                            }
                            if (this.state.pageQuery.trim().length < 2) {
                                return 'Type at least 2 characters to search.';
                            }
                            if (loadingPages) {
                                return 'Searching pages...';
                            }
                            return 'No matching pages found.';
                        }}
                    />
                    {Boolean(error) && (
                        <p className='alert alert-danger'>
                            <i className='fa fa-warning' title='Warning Icon'/>
                            <span> {error}</span>
                        </p>
                    )}
                </Modal.Body>
                <Modal.Footer style={modalStyles.footer}>
                    <Button
                        type='button'
                        bsStyle='link'
                        onClick={this.handleClose}
                    >
                        {'Cancel'}
                    </Button>
                    <Button
                        type='submit'
                        bsStyle='primary'
                        onClick={this.handleSubmit}
                        disabled={saving}
                    >
                        {saving && <span className='fa fa-spinner fa-fw fa-pulse spinner'/>}
                        {'Add Comment'}
                    </Button>
                </Modal.Footer>
            </Modal>
        );
    }
}
