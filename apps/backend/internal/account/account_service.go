package account

// 账号服务层：负责账号生命周期相关的业务逻辑，包括注册、登录、改密、
// 改名、登出、刷新令牌以及个人资料更新。所有数据库操作都委托给 AccountRepository。

import (
	"context"
	"errors"
	"strings"

	"tiny-feed/internal/auth"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AccountService 是账号业务的服务端聚合根。
type AccountService struct {
	repo *AccountRepository
}

// NewAccountService 构造账号服务。repo 不能为空。
func NewAccountService(repo *AccountRepository) *AccountService {
	return &AccountService{repo: repo}
}

// CreateAccount 处理账号注册请求。
// 流程：
//  1. 校验用户名和密码非空；
//  2. 用 bcrypt 对明文密码进行哈希（DefaultCost 兼顾安全与性能）；
//  3. 写入数据库。并发注册同一用户名时，依赖 username 的 unique 索引兜底，
//     失败时把 MySQL 1062 错误转成业务可读的"username already taken"，
//     避免"先查后插"的 TOCTOU race。
// 返回成功创建的账号实体（不含密码哈希）。
func (s *AccountService) CreateAccount(ctx context.Context, req *CreateAccountRequest) (*Account, error) {
	// 先把首尾空白去掉，避免出现" bob"和"bob"被当成两个账号的情况。
	username := strings.TrimSpace(req.Username)
	if username == "" || req.Password == "" {
		return nil, errors.New("username and password are required")
	}
	// bcrypt 单向哈希，存库不存明文。
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	acc := &Account{
		Username: username,
		Password: string(hashed),
	}
	if err := s.repo.CreateAccount(ctx, acc); err != nil {
		// 并发注册同 username：DB unique 索引会拒绝，转换成业务错误。
		if isDuplicateKeyError(err) {
			return nil, errors.New("username already taken")
		}
		return nil, err
	}
	return acc, nil
}

// Login 处理账号登录请求。
// 流程：
//  1. 按用户名查找账号；
//  2. 用 bcrypt 校验密码（不区分"用户不存在"和"密码错误"，对外统一返回 invalid）；
//  3. 签发 JWT（短期访问令牌）和 refresh token（用于无感续期）；
//  4. 把新令牌写回数据库，覆盖旧的 token 字段，从而实现单点登出语义；
//  5. 把令牌和账号基本信息返回给调用方。
func (s *AccountService) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
	acc, err := s.repo.FindByUsername(ctx, req.Username)
	if err != nil {
		// 用户不存在也返回"用户名或密码错误"，避免泄漏账号是否存在的信息。
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("invalid username or password")
		}
		return nil, err
	}
	// 校验密码哈希。
	if err := bcrypt.CompareHashAndPassword([]byte(acc.Password), []byte(req.Password)); err != nil {
		return nil, errors.New("invalid username or password")
	}
	// 生成访问令牌（JWT）。
	token, err := auth.GenerateToken(acc.ID, acc.Username)
	if err != nil {
		return nil, err
	}
	// 生成刷新令牌（随机串）。
	refresh, err := auth.GenerateRefreshToken(acc.ID)
	if err != nil {
		return nil, err
	}
	// 把最新令牌写回 DB，旧的自动失效（用于 jwt 中间件里的"当前 token 一致性"校验）。
	if err := s.repo.Login(ctx, acc.ID, token, refresh); err != nil {
		return nil, err
	}
	return &LoginResponse{
		Token:        token,
		RefreshToken: refresh,
		AccountID:    acc.ID,
		Username:     acc.Username,
	}, nil
}

// ChangePassword 修改账号密码。
// 流程：按用户名找到账号 → 校验旧密码 → 用新密码重新哈希 → 写回。
// 出于一致性考虑，这里也走 RenameWithToken 之外的独立写入路径（不改用户名）。
func (s *AccountService) ChangePassword(ctx context.Context, req *ChangePasswordRequest) error {
	acc, err := s.repo.FindByUsername(ctx, req.Username)
	if err != nil {
		return errors.New("user not found")
	}
	// 必须先验证旧密码通过，才允许改。
	if err := bcrypt.CompareHashAndPassword([]byte(acc.Password), []byte(req.OldPassword)); err != nil {
		return errors.New("old password is incorrect")
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.repo.ChangePassword(ctx, acc.ID, string(hashed))
}

// FindByID 按主键查询账号的"对外可见"字段（不包含密码、token 等敏感字段）。
func (s *AccountService) FindByID(ctx context.Context, id uint) (*FindByIDResponse, error) {
	acc, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &FindByIDResponse{
		ID:        acc.ID,
		Username:  acc.Username,
		AvatarURL: acc.AvatarURL,
		Bio:       acc.Bio,
	}, nil
}

// FindByUsername 按用户名查询账号的"对外可见"字段。
func (s *AccountService) FindByUsername(ctx context.Context, username string) (*FindByUsernameResponse, error) {
	acc, err := s.repo.FindByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	return &FindByUsernameResponse{ID: acc.ID, Username: acc.Username}, nil
}

// Rename 修改用户名。
// 因为 JWT 的 claims 里包含 username，改名后必须重新签发令牌，否则旧 token 立刻失效。
// 借助 repo 的 RenameWithToken 在同一事务里同时改 username 和 token 字段，保证一致性。
// 如果新名字已被占用（并发改名撞名），依赖 DB unique 索引兜底并转业务错误。
func (s *AccountService) Rename(ctx context.Context, id uint, newUsername string) error {
	newUsername = strings.TrimSpace(newUsername)
	if newUsername == "" {
		return errors.New("username is required")
	}
	// 用新用户名重新签发 JWT。
	token, err := auth.GenerateToken(id, newUsername)
	if err != nil {
		return err
	}
	// 事务内同步更新 username 和 token，避免出现"新名字但旧 token"的中间态。
	if err := s.repo.RenameWithToken(ctx, id, newUsername, token); err != nil {
		if isDuplicateKeyError(err) {
			return errors.New("username already taken")
		}
		return err
	}
	return nil
}

// isDuplicateKeyError 判断 gorm/MySQL 错误是否是唯一键冲突（1062 / 23000）。
// 同时兼容 gorm.ErrDuplicatedKey（GORM 2.x 起的标准化错误）。
// 注册/改名时并发撞名靠这个判断把 500 转成 4xx。
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	// 不同 driver 的错误消息格式不一样，做一次字符串兜底匹配。
	msg := err.Error()
	return strings.Contains(msg, "Error 1062") || strings.Contains(msg, "Duplicate entry")
}

// Logout 把账号的 token 字段清空，jwt 中间件在下次校验时会因为查不到 token 而拒绝。
// 这是一种基于"持久化 token 黑名单"的简单登出方案。
func (s *AccountService) Logout(ctx context.Context, id uint) error {
	return s.repo.Logout(ctx, id)
}

// UpdateProfile 更新个人资料（头像 URL、个性签名）。
// 只更新请求中实际带了值的字段，未提供的字段保持原样。
func (s *AccountService) UpdateProfile(ctx context.Context, id uint, req *UpdateProfileRequest) error {
	updates := map[string]interface{}{}
	if req.AvatarURL != "" {
		updates["avatar_url"] = req.AvatarURL
	}
	if req.Bio != "" {
		updates["bio"] = req.Bio
	}
	// 没东西可改就直接返回。
	if len(updates) == 0 {
		return nil
	}
	return s.repo.UpdateFields(ctx, id, updates)
}

// IssueTokens 给已登录的账号重新签发一对令牌（access + refresh）。
// 用于 /account/refresh 接口续期场景。
func (s *AccountService) IssueTokens(ctx context.Context, accountID uint, username string) (string, string, error) {
	token, err := auth.GenerateToken(accountID, username)
	if err != nil {
		return "", "", err
	}
	refresh, err := auth.GenerateRefreshToken(accountID)
	if err != nil {
		return "", "", err
	}
	// 把新 token 写回 DB 覆盖旧值，旧 token 立即失效。
	if err := s.repo.Login(ctx, accountID, token, refresh); err != nil {
		return "", "", err
	}
	return token, refresh, nil
}

// ResolveRefreshToken 根据 refresh token 反查账号。
// 找不到时统一返回 invalid refresh token，对外不暴露"已过期"或"已使用"等细节。
func (s *AccountService) ResolveRefreshToken(ctx context.Context, refreshToken string) (uint, string, error) {
	if refreshToken == "" {
		return 0, "", errors.New("refresh token is empty")
	}
	var acc Account
	// 直接走 repo 的 db 句柄查 refresh_token 字段（repo 暂未封装此方法）。
	if err := s.repo.db.WithContext(ctx).Where("refresh_token = ?", refreshToken).First(&acc).Error; err != nil {
		return 0, "", errors.New("invalid refresh token")
	}
	return acc.ID, acc.Username, nil
}
