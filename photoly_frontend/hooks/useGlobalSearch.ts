import { useGetChildrenQuery, useGetRootQuery } from "../redux/api/namespaceSlice";
import { skipToken } from "@reduxjs/toolkit/query";
import { useGetPhotoByAlbumQuery, useGetPhotoByNamespaceQuery, useGetPhotoByTagsQuery } from "../redux/api/photoSlice";
import { useAppDispatch, useAppSelector } from "../redux/hooks";
import { ISearchFilter } from "../redux/states/searchFilterSlice";
import { changeCurrDisplay } from "../redux/states/mainDisplaySlice";

interface IPhoto {
  id: number;
  name: string;
  format: string;
  uploaddate: string;
  visibility: boolean; // why is this boolean?
}

interface IFolder {
  id: number;
  name: string;
  parentId: number;
  userId: number;
}

interface ISearchData {
  path: IFolder[];
  current: IFolder;
  folders: IFolder[];
  photos: IPhoto[];
}

const defaultFolder: IFolder = {
  id: -1,
  name: "/",
  parentId: -1,
  userId: -1
};

const initSearchDataState: ISearchData = {
  path: [defaultFolder],
  current: defaultFolder,
  folders: [],
  photos: []
};

function isFilterClear(filter: ISearchFilter) {
  return filter.albumId === -1 && filter.tagIds.length === 0;
}

function isTagApply(filter: ISearchFilter) {
  return filter.tagIds.length > 0;
}

function isAlbumApply(filter: ISearchFilter) {
  return filter.albumId !== -1;
}

export default function useGlobalSearch(): ISearchData {
  const curFilter = useAppSelector((state) => state.searchFilter);
  const dispatch = useAppDispatch();

  // If no filter apply, then query the root folder
  const {
    data: root_dir,
    isSuccess: isRootSuccess,
    isUninitialized: isRootSkipped
  } = useGetRootQuery(isFilterClear(curFilter) ? undefined : skipToken);

  const {
    data: children_dirs,
    isSuccess: isChildrenSuccess
  } = useGetChildrenQuery(isRootSuccess ? curFilter.folderId === -1 ? root_dir.nsId : curFilter.folderId : skipToken);
  const {
    data: photos,
    isSuccess: isPhotoSuccess
  } = useGetPhotoByNamespaceQuery(isRootSuccess ? curFilter.folderId === -1 ? root_dir.nsId : curFilter.folderId : skipToken);

  // If tag filter apply
  const {
    data: tag_photos,
    isSuccess: isTagPhotoSuccess
  } = useGetPhotoByTagsQuery(isTagApply(curFilter) ? curFilter.tagIds : skipToken);

  // If album filter apply
  const {
    data: album_photos,
    isSuccess: isAlbumPhotoSuccess
  } = useGetPhotoByAlbumQuery(isAlbumApply(curFilter) ? curFilter.albumId : skipToken);

  if (isFilterClear(curFilter) && isChildrenSuccess && isPhotoSuccess && isRootSuccess) {
    let photoData: IPhoto[] = [];
    photos.forEach((e) => {
      photoData.push({
        id: e.photoId,
        name: e.photoName,
        format: e.format,
        uploaddate: e.uploadDate,
        visibility: !!e.visibility
      });
    });

    let folderData: IFolder[] = [];
    children_dirs.forEach((e) => {
      folderData.push({
        id: e.nsId,
        name: e.nsName,
        parentId: e.nsParentId,
        userId: e.userId
      });
    });

    let rootFolder: IFolder = {
      id: root_dir.nsId, name: root_dir.nsName, parentId: root_dir.nsParentId, userId: root_dir.userId
    };

    dispatch(changeCurrDisplay({
      path: [rootFolder],
      current: rootFolder,
      photos: photoData,
      folders: folderData
    }));
    return {
      path: [rootFolder],
      current: rootFolder,
      photos: photoData,
      folders: folderData
    };
  }

  if (isTagApply(curFilter) && isTagPhotoSuccess) {
    let photoData: IPhoto[] = [];
    tag_photos.forEach((e) => {
      photoData.push({
        id: e.photoId,
        name: e.photoName,
        format: e.format,
        uploaddate: e.uploadDate,
        visibility: !!e.visibility
      });
    });
    dispatch(changeCurrDisplay({
      path: [defaultFolder],
      current: defaultFolder,
      photos: photoData,
      folders: []
    }));
    return {
      path: [defaultFolder],
      current: defaultFolder,
      photos: photoData,
      folders: []
    };
  }

  if (isAlbumApply(curFilter) && isAlbumPhotoSuccess) {
    let photoData: IPhoto[] = [];
    album_photos.forEach((e) => {
      photoData.push({
        id: e.photoId,
        name: e.photoName,
        format: e.format,
        uploaddate: e.uploadDate,
        visibility: !!e.visibility
      });
    });
    dispatch(changeCurrDisplay({
      path: [defaultFolder],
      current: defaultFolder,
      photos: photoData,
      folders: []
    }));
    return {
      path: [defaultFolder],
      current: defaultFolder,
      photos: photoData,
      folders: []
    };
  }

  return initSearchDataState;
}